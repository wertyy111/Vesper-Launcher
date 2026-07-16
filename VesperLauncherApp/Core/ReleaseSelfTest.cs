using System;
using System.Collections.Generic;
using System.IO;
using System.Linq;
using System.Reflection;
using System.Runtime.InteropServices;
using System.Text;
using System.Text.Json;
using VesperLauncher.Launcher;
using VesperLauncher.PhotinoHost;
using VesperLauncher.Platform;

namespace VesperLauncher.Core;

internal static class ReleaseSelfTest
{
    private static readonly string[] ForbiddenExtensions = [".cs", ".csproj", ".pdb", ".map", ".sln"];
    private static readonly string[] ForbiddenUserStateFiles =
    [
        "account-state.json",
        "account-friends-state.json",
        "saved-usernames.json"
    ];
    private static readonly byte[][] ForbiddenSecretMarkers =
    [
        BuildMarker("gh", "p_"),
        BuildMarker("github_", "pat_"),
        BuildMarker("-----BEGIN ", "PRIVATE KEY-----"),
        BuildMarker("MACOS_CERTIFICATE_", "P12_BASE64"),
        BuildMarker("VESPER_RELEASE_", "TOKEN")
    ];

    public static bool IsRequested(IEnumerable<string> args) =>
        args.Any(arg => string.Equals(arg, "--self-test", StringComparison.OrdinalIgnoreCase));

    public static int Run(IReadOnlyList<string> args)
    {
        var failures = new List<string>();
        var baseDirectory = AppContext.BaseDirectory;
        var expectedRid = ReadArgument(args, "--expect-rid");
        var reportPath = ReadArgument(args, "--self-test-report");

        CheckExpectedRuntime(expectedRid, failures);
        CheckRequiredFiles(baseDirectory, failures);
        CheckUpdateConfiguration(baseDirectory, failures);
        CheckReleaseHygiene(baseDirectory, failures);
        CheckForSecretMarkers(baseDirectory, failures);
        CheckMutableGameStorage(failures);
        CheckBridgeSerialization(failures, HasArgument(args, "--expect-clean-profile"));

        if (!string.IsNullOrWhiteSpace(reportPath))
        {
            try
            {
                var fullReportPath = Path.GetFullPath(reportPath);
                Directory.CreateDirectory(Path.GetDirectoryName(fullReportPath) ?? baseDirectory);
                File.WriteAllText(
                    fullReportPath,
                    failures.Count == 0 ? "PASS" : "FAIL" + Environment.NewLine + string.Join(Environment.NewLine, failures));
            }
            catch (Exception ex)
            {
                failures.Add("Unable to write self-test report: " + ex.Message);
            }
        }

        if (failures.Count == 0)
        {
            Console.WriteLine($"Vesper release self-test passed ({RuntimeInformation.RuntimeIdentifier}).");
            return 0;
        }

        Console.Error.WriteLine("Vesper release self-test failed:");
        foreach (var failure in failures)
        {
            Console.Error.WriteLine("- " + failure);
        }

        return 1;
    }

    private static void CheckExpectedRuntime(string? expectedRid, ICollection<string> failures)
    {
        if (string.IsNullOrWhiteSpace(expectedRid))
        {
            return;
        }

        var actualRid = RuntimeInformation.RuntimeIdentifier;
        if (!actualRid.Equals(expectedRid, StringComparison.OrdinalIgnoreCase))
        {
            failures.Add($"Runtime mismatch: expected {expectedRid}, actual {actualRid}.");
        }
    }

    private static void CheckRequiredFiles(string root, ICollection<string> failures)
    {
        string[] requiredFiles =
        [
            "VesperLauncher.dll",
            "VesperLauncher.deps.json",
            "VesperLauncher.runtimeconfig.json",
            "update-config.json",
            "LICENSE.txt",
            Path.Combine("Assets", "vesper-logo.png"),
            Path.Combine("UserInterface", "dist", "index.html")
        ];

        foreach (var relativePath in requiredFiles)
        {
            var path = Path.Combine(root, relativePath);
            if (!File.Exists(path) || new FileInfo(path).Length == 0)
            {
                failures.Add($"Required release file is missing or empty: {relativePath} (base: {root})");
            }
        }

        var nativeLibrary = OperatingSystem.IsWindows()
            ? "Photino.Native.dll"
            : OperatingSystem.IsMacOS()
                ? "Photino.Native.dylib"
                : "Photino.Native.so";
        if (!File.Exists(Path.Combine(root, nativeLibrary)))
        {
            failures.Add($"Photino native library is missing: {nativeLibrary}");
        }
    }

    private static void CheckUpdateConfiguration(string root, ICollection<string> failures)
    {
        var path = Path.Combine(root, "update-config.json");
        if (!File.Exists(path))
        {
            return;
        }

        try
        {
            using var document = JsonDocument.Parse(File.ReadAllBytes(path));
            var config = document.RootElement;
            if (!config.TryGetProperty("SourceUrl", out var source) ||
                !Uri.TryCreate(source.GetString(), UriKind.Absolute, out var uri) ||
                uri.Scheme != Uri.UriSchemeHttps)
            {
                failures.Add("update-config.json must contain an HTTPS SourceUrl.");
            }
        }
        catch (Exception ex)
        {
            failures.Add("update-config.json is invalid: " + ex.Message);
        }
    }

    private static void CheckReleaseHygiene(string root, ICollection<string> failures)
    {
        foreach (var path in Directory.EnumerateFiles(root, "*", SearchOption.AllDirectories))
        {
            var relativePath = Path.GetRelativePath(root, path);
            if (ForbiddenExtensions.Contains(Path.GetExtension(path), StringComparer.OrdinalIgnoreCase))
            {
                failures.Add("Development artifact leaked into release: " + relativePath);
            }

            if (ForbiddenUserStateFiles.Contains(Path.GetFileName(path), StringComparer.OrdinalIgnoreCase))
            {
                failures.Add("User account state leaked into release: " + relativePath);
            }
        }

        if (Assembly.GetExecutingAssembly().GetCustomAttribute<AssemblyInformationalVersionAttribute>() is null)
        {
            failures.Add("Assembly informational version is missing.");
        }
    }

    private static void CheckForSecretMarkers(string root, ICollection<string> failures)
    {
        var candidates = Directory.EnumerateFiles(root, "*", SearchOption.AllDirectories)
            .Where(path => new FileInfo(path).Length <= 32 * 1024 * 1024)
            .Where(path => Path.GetExtension(path) is ".dll" or ".json" or ".html" or ".js" or ".css" or ".txt");

        foreach (var path in candidates)
        {
            var bytes = File.ReadAllBytes(path);
            foreach (var marker in ForbiddenSecretMarkers)
            {
                if (bytes.AsSpan().IndexOf(marker) >= 0)
                {
                    failures.Add($"Possible secret marker found in {Path.GetRelativePath(root, path)}: {Encoding.ASCII.GetString(marker)}");
                }
            }
        }
    }

    private static void CheckBridgeSerialization(ICollection<string> failures, bool expectCleanProfile)
    {
        try
        {
            using var backendHost = LauncherBackendHostFactory.CreateCurrent();
            var snapshot = backendHost.GetSnapshotAsync().GetAwaiter().GetResult();
            var envelopeJson = JsonSerializer.Serialize(new
            {
                type = "snapshot",
                data = snapshot
            });

            using var document = JsonDocument.Parse(envelopeJson);
            var root = document.RootElement;
            var hasData = root.TryGetProperty("data", out var data);
            if (!root.TryGetProperty("type", out var type) ||
                !string.Equals(type.GetString(), "snapshot", StringComparison.Ordinal) ||
                !hasData ||
                data.ValueKind != JsonValueKind.Object ||
                !data.TryGetProperty("launcher", out _))
            {
                failures.Add("Bridge snapshot envelope is incomplete after serialization.");
            }

            if (expectCleanProfile &&
                data.TryGetProperty("launcher", out var launcher) &&
                launcher.TryGetProperty("account", out var account) &&
                account.TryGetProperty("hasAuthenticatedSession", out var hasAuthenticatedSession) &&
                hasAuthenticatedSession.ValueKind == JsonValueKind.True)
            {
                failures.Add("A clean launcher profile unexpectedly contains an authenticated account session.");
            }
        }
        catch (Exception ex)
        {
            failures.Add("Bridge snapshot serialization failed: " + ex.Message);
        }
    }

    private static void CheckMutableGameStorage(ICollection<string> failures)
    {
        try
        {
            var storageDirectory = new MinecraftLauncherService().GetBaseStorageDirectory();
            if (!MutableStoragePathResolver.TryEnsureWritableDirectory(storageDirectory))
            {
                failures.Add("Resolved game storage directory is not writable: " + storageDirectory);
            }

            var versionDirectory = Path.GetFullPath(AppContext.BaseDirectory)
                .TrimEnd(Path.DirectorySeparatorChar, Path.AltDirectorySeparatorChar);
            var versionDirectoryName = Path.GetFileName(versionDirectory);
            var storageFullPath = Path.GetFullPath(storageDirectory)
                .TrimEnd(Path.DirectorySeparatorChar, Path.AltDirectorySeparatorChar);
            var comparison = OperatingSystem.IsWindows() ? StringComparison.OrdinalIgnoreCase : StringComparison.Ordinal;
            if ((versionDirectoryName.Equals("current", StringComparison.OrdinalIgnoreCase) ||
                 versionDirectoryName.StartsWith("app-", StringComparison.OrdinalIgnoreCase)) &&
                storageFullPath.StartsWith(versionDirectory + Path.DirectorySeparatorChar, comparison))
            {
                failures.Add("Mutable game storage must not be placed inside an update-managed version directory.");
            }
        }
        catch (Exception ex)
        {
            failures.Add("Game storage validation failed: " + ex.Message);
        }
    }

    private static string? ReadArgument(IReadOnlyList<string> args, string name)
    {
        for (var index = 0; index < args.Count - 1; index++)
        {
            if (args[index].Equals(name, StringComparison.OrdinalIgnoreCase))
            {
                return args[index + 1];
            }
        }

        return null;
    }

    private static bool HasArgument(IReadOnlyList<string> args, string name) =>
        args.Any(arg => arg.Equals(name, StringComparison.OrdinalIgnoreCase));

    private static byte[] BuildMarker(string prefix, string suffix) =>
        Encoding.ASCII.GetBytes(string.Concat(prefix, suffix));
}
