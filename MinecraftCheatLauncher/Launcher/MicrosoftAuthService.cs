using System;
using System.Collections.Generic;
using System.Diagnostics;
using System.IO;
using System.Net.Http;
using System.Net.Http.Json;
using System.Text;
using System.Text.Json;
using System.Text.Json.Serialization;
using System.Threading;
using System.Threading.Tasks;
using MinecraftCheatLauncher.Platform;

namespace MinecraftCheatLauncher.Launcher;

/// <summary>
/// Authenticates a player via Microsoft Account (OAuth 2.0 Device Code Flow)
/// and then exchanges the token through Xbox Live → XSTS → Minecraft.
/// </summary>
public sealed class MicrosoftAuthService
{
    // Azure application registered for open-source Minecraft launchers (PrismLauncher).
    // Replace with your own Azure app Client ID if needed.
    private const string AzureClientId = "c36a9fb6-4f2a-41ff-90bd-ae7cc92031eb";
    private const string AzureTenantId = "consumers";
    private const string MsaScope = "XboxLive.signin offline_access";

    private static readonly string CacheFilePath = LauncherDataPaths.GetDataFilePath("msa-token.json");

    private static readonly HttpClient Http = new() { Timeout = TimeSpan.FromMinutes(5) };

    // ──────────────────────────────────────────────────────────────────
    // Public API
    // ──────────────────────────────────────────────────────────────────

    /// <summary>
    /// Tries to load a cached session and refresh it silently.
    /// Returns null if no cached session exists or the refresh fails.
    /// </summary>
    public async Task<MicrosoftSession?> TryLoadCachedSessionAsync(CancellationToken ct = default)
    {
        var cached = TryLoadCache();
        if (cached is null) return null;

        try
        {
            var refreshed = await RefreshMsaTokenAsync(cached.MsaRefreshToken, ct);
            var session = await ExchangeForMinecraftSessionAsync(refreshed.AccessToken, ct);
            SaveCache(cached with { MsaRefreshToken = refreshed.RefreshToken ?? cached.MsaRefreshToken });
            return session;
        }
        catch
        {
            return null;
        }
    }

    /// <summary>
    /// Starts the Device Code Flow. Returns the URL + code to show the user,
    /// then continuously polls until authenticated, then returns the session.
    /// </summary>
    public async Task<(MicrosoftSession Session, Action StopPolling)> StartDeviceCodeFlowAsync(
        Action<DeviceCodeInfo> onDeviceCode,
        CancellationToken ct = default)
    {
        using var cts = CancellationTokenSource.CreateLinkedTokenSource(ct);

        var deviceCode = await RequestDeviceCodeAsync(cts.Token);
        onDeviceCode(deviceCode);

        var msaToken = await PollForMsaTokenAsync(deviceCode, cts.Token);
        var session = await ExchangeForMinecraftSessionAsync(msaToken.AccessToken, cts.Token);

        SaveCache(new MsaTokenCache(msaToken.RefreshToken!));
        return (session, () => { try { cts.Cancel(); } catch { /**/ } });
    }

    // ──────────────────────────────────────────────────────────────────
    // Step 1 – Microsoft Device Code
    // ──────────────────────────────────────────────────────────────────

    private static async Task<DeviceCodeInfo> RequestDeviceCodeAsync(CancellationToken ct)
    {
        var url = $"https://login.microsoftonline.com/{AzureTenantId}/oauth2/v2.0/devicecode";
        var body = new FormUrlEncodedContent(new Dictionary<string, string>
        {
            ["client_id"] = AzureClientId,
            ["scope"] = MsaScope
        });

        using var response = await Http.PostAsync(url, body, ct);
        response.EnsureSuccessStatusCode();

        using var doc = JsonDocument.Parse(await response.Content.ReadAsStringAsync(ct));
        var root = doc.RootElement;

        return new DeviceCodeInfo(
            root.GetProperty("device_code").GetString()!,
            root.GetProperty("user_code").GetString()!,
            root.GetProperty("verification_uri").GetString()!,
            root.GetProperty("interval").GetInt32(),
            root.GetProperty("expires_in").GetInt32());
    }

    // ──────────────────────────────────────────────────────────────────
    // Step 2 – Poll for MSA token
    // ──────────────────────────────────────────────────────────────────

    private static async Task<MsaTokenResponse> PollForMsaTokenAsync(DeviceCodeInfo deviceCode, CancellationToken ct)
    {
        var url = $"https://login.microsoftonline.com/{AzureTenantId}/oauth2/v2.0/token";
        var deadline = DateTimeOffset.UtcNow.AddSeconds(deviceCode.ExpiresIn);
        var interval = TimeSpan.FromSeconds(Math.Max(deviceCode.Interval, 5));

        while (DateTimeOffset.UtcNow < deadline)
        {
            ct.ThrowIfCancellationRequested();
            await Task.Delay(interval, ct);

            var body = new FormUrlEncodedContent(new Dictionary<string, string>
            {
                ["grant_type"] = "urn:ietf:params:oauth:grant-type:device_code",
                ["client_id"] = AzureClientId,
                ["device_code"] = deviceCode.DeviceCode
            });

            using var response = await Http.PostAsync(url, body, ct);
            var json = await response.Content.ReadAsStringAsync(ct);
            using var doc = JsonDocument.Parse(json);
            var root = doc.RootElement;

            if (root.TryGetProperty("error", out var errorElement))
            {
                var error = errorElement.GetString();
                if (error == "authorization_pending" || error == "slow_down") continue;
                throw new InvalidOperationException($"Ошибка авторизации Microsoft: {error}");
            }

            return new MsaTokenResponse(
                root.GetProperty("access_token").GetString()!,
                root.TryGetProperty("refresh_token", out var rt) ? rt.GetString() : null);
        }

        throw new TimeoutException("Время ожидания авторизации Microsoft истекло.");
    }

    // ──────────────────────────────────────────────────────────────────
    // Step 3 – Refresh MSA token
    // ──────────────────────────────────────────────────────────────────

    private static async Task<MsaTokenResponse> RefreshMsaTokenAsync(string refreshToken, CancellationToken ct)
    {
        var url = $"https://login.microsoftonline.com/{AzureTenantId}/oauth2/v2.0/token";
        var body = new FormUrlEncodedContent(new Dictionary<string, string>
        {
            ["grant_type"] = "refresh_token",
            ["client_id"] = AzureClientId,
            ["refresh_token"] = refreshToken
        });

        using var response = await Http.PostAsync(url, body, ct);
        response.EnsureSuccessStatusCode();

        using var doc = JsonDocument.Parse(await response.Content.ReadAsStringAsync(ct));
        var root = doc.RootElement;

        return new MsaTokenResponse(
            root.GetProperty("access_token").GetString()!,
            root.TryGetProperty("refresh_token", out var rt) ? rt.GetString() : null);
    }

    // ──────────────────────────────────────────────────────────────────
    // Step 4 – MSA → Xbox Live → XSTS → Minecraft
    // ──────────────────────────────────────────────────────────────────

    private static async Task<MicrosoftSession> ExchangeForMinecraftSessionAsync(string msaAccessToken, CancellationToken ct)
    {
        // 4a. Xbox Live
        var xblToken = await AuthenticateXboxLiveAsync(msaAccessToken, ct);

        // 4b. XSTS
        var (xstsToken, userHash) = await AuthenticateXstsAsync(xblToken, ct);

        // 4c. Minecraft
        var mcToken = await AuthenticateMinecraftAsync(xstsToken, userHash, ct);

        // 4d. Profile
        var (uuid, username) = await GetMinecraftProfileAsync(mcToken, ct);

        return new MicrosoftSession(uuid, username, mcToken);
    }

    private static async Task<string> AuthenticateXboxLiveAsync(string msaToken, CancellationToken ct)
    {
        var payload = new
        {
            Properties = new
            {
                AuthMethod = "RPS",
                SiteName = "user.auth.xboxlive.com",
                RpsTicket = $"d={msaToken}"
            },
            RelyingParty = "http://auth.xboxlive.com",
            TokenType = "JWT"
        };

        using var response = await Http.PostAsJsonAsync("https://user.auth.xboxlive.com/user/authenticate", payload, ct);
        response.EnsureSuccessStatusCode();

        using var doc = JsonDocument.Parse(await response.Content.ReadAsStringAsync(ct));
        return doc.RootElement.GetProperty("Token").GetString()!;
    }

    private static async Task<(string Token, string UserHash)> AuthenticateXstsAsync(string xblToken, CancellationToken ct)
    {
        var payload = new
        {
            Properties = new
            {
                SandboxId = "RETAIL",
                UserTokens = new[] { xblToken }
            },
            RelyingParty = "rp://api.minecraftservices.com/",
            TokenType = "JWT"
        };

        using var response = await Http.PostAsJsonAsync("https://xsts.auth.xboxlive.com/xsts/authorize", payload, ct);

        if (!response.IsSuccessStatusCode)
        {
            var json = await response.Content.ReadAsStringAsync(ct);
            using var doc = JsonDocument.Parse(json);
            var xerr = doc.RootElement.TryGetProperty("XErr", out var e) ? e.GetInt64() : 0;
            var msg = xerr switch
            {
                2148916233 => "У этого Microsoft-аккаунта нет привязанного аккаунта Xbox. Войдите на xbox.com и создайте его.",
                2148916235 => "Xbox заблокирован в вашей стране.",
                2148916238 => "Аккаунт является детским. Требуется разрешение родителя.",
                _ => $"Ошибка XSTS (XErr={xerr})."
            };
            throw new InvalidOperationException(msg);
        }

        using var okDoc = JsonDocument.Parse(await response.Content.ReadAsStringAsync(ct));
        var root = okDoc.RootElement;
        var token = root.GetProperty("Token").GetString()!;
        var userHash = root.GetProperty("DisplayClaims")
                           .GetProperty("xui")[0]
                           .GetProperty("uhs")
                           .GetString()!;

        return (token, userHash);
    }

    private static async Task<string> AuthenticateMinecraftAsync(string xstsToken, string userHash, CancellationToken ct)
    {
        var payload = new { identityToken = $"XBL3.0 x={userHash};{xstsToken}" };

        using var response = await Http.PostAsJsonAsync(
            "https://api.minecraftservices.com/authentication/login_with_xbox", payload, ct);
        response.EnsureSuccessStatusCode();

        using var doc = JsonDocument.Parse(await response.Content.ReadAsStringAsync(ct));
        return doc.RootElement.GetProperty("access_token").GetString()!;
    }

    private static async Task<(string Uuid, string Username)> GetMinecraftProfileAsync(string mcToken, CancellationToken ct)
    {
        using var request = new HttpRequestMessage(HttpMethod.Get, "https://api.minecraftservices.com/minecraft/profile");
        request.Headers.Authorization = new System.Net.Http.Headers.AuthenticationHeaderValue("Bearer", mcToken);

        using var response = await Http.SendAsync(request, ct);

        if (!response.IsSuccessStatusCode)
        {
            throw new InvalidOperationException("У этого аккаунта нет купленного Minecraft Java Edition.");
        }

        using var doc = JsonDocument.Parse(await response.Content.ReadAsStringAsync(ct));
        var root = doc.RootElement;

        var rawUuid = root.GetProperty("id").GetString()!;
        var uuid = $"{rawUuid[..8]}-{rawUuid[8..12]}-{rawUuid[12..16]}-{rawUuid[16..20]}-{rawUuid[20..]}";
        var username = root.GetProperty("name").GetString()!;

        return (uuid, username);
    }

    // ──────────────────────────────────────────────────────────────────
    // Token cache
    // ──────────────────────────────────────────────────────────────────

    private static MsaTokenCache? TryLoadCache()
    {
        foreach (var cachePath in LauncherDataPaths.GetDataFileCandidates("msa-token.json"))
        {
            try
            {
                if (!File.Exists(cachePath))
                {
                    continue;
                }

                var json = File.ReadAllText(cachePath);
                return JsonSerializer.Deserialize<MsaTokenCache>(json);
            }
            catch
            {
                // Try the next compatibility path.
            }
        }

        return null;
    }

    private static void SaveCache(MsaTokenCache cache)
    {
        try
        {
            Directory.CreateDirectory(Path.GetDirectoryName(CacheFilePath)!);
            File.WriteAllText(CacheFilePath, JsonSerializer.Serialize(cache, new JsonSerializerOptions { WriteIndented = true }));
        }
        catch { /**/ }
    }

    public static void ClearCache()
    {
        foreach (var cachePath in LauncherDataPaths.GetDataFileCandidates("msa-token.json"))
        {
            try { File.Delete(cachePath); } catch { /**/ }
        }
    }
}

// ──────────────────────────────────────────────────────────────────────
// Data models
// ──────────────────────────────────────────────────────────────────────

public sealed record MicrosoftSession(string Uuid, string Username, string AccessToken);

public sealed record DeviceCodeInfo(
    string DeviceCode,
    string UserCode,
    string VerificationUri,
    int Interval,
    int ExpiresIn);

internal sealed record MsaTokenResponse(string AccessToken, string? RefreshToken);

internal sealed record MsaTokenCache([property: JsonPropertyName("refresh_token")] string MsaRefreshToken);
