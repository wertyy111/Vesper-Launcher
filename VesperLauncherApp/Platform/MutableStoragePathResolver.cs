using System;
using System.Collections.Generic;
using System.IO;
using System.Linq;

namespace VesperLauncher.Platform;

internal static class MutableStoragePathResolver
{
    private const string LauncherDirectoryName = "Vesper";

    public static string ResolveGameDataRoot(string userDataDirectory)
    {
        var baseDirectory = Path.GetFullPath(AppContext.BaseDirectory);
        var stableApplicationRoot = ResolveStableApplicationRoot(baseDirectory);
        var candidates = new List<string>
        {
            Path.Combine(stableApplicationRoot, "GameData"),
            Path.Combine(userDataDirectory, LauncherDirectoryName, "GameData")
        };

        var localApplicationData = Environment.GetFolderPath(Environment.SpecialFolder.LocalApplicationData);
        if (!string.IsNullOrWhiteSpace(localApplicationData))
        {
            candidates.Add(Path.Combine(localApplicationData, LauncherDirectoryName, "GameData"));
        }

        candidates.Add(Path.Combine(Path.GetTempPath(), LauncherDirectoryName, "GameData"));

        var comparer = OperatingSystem.IsWindows() ? StringComparer.OrdinalIgnoreCase : StringComparer.Ordinal;
        foreach (var candidate in candidates.Where(path => !string.IsNullOrWhiteSpace(path)).Distinct(comparer))
        {
            if (TryEnsureWritableDirectory(candidate))
            {
                return candidate;
            }
        }

        throw new IOException("?? ??????? ??????????? ????????? ??? ?????? ????? ??????? ??????.");
    }

    public static bool TryEnsureWritableDirectory(string directoryPath)
    {
        string? probePath = null;
        try
        {
            Directory.CreateDirectory(directoryPath);
            probePath = Path.Combine(directoryPath, $".vesper-write-probe-{Environment.ProcessId}-{Guid.NewGuid():N}");
            using (new FileStream(probePath, FileMode.CreateNew, FileAccess.Write, FileShare.None, 1, FileOptions.DeleteOnClose))
            {
            }

            return true;
        }
        catch
        {
            return false;
        }
        finally
        {
            if (!string.IsNullOrWhiteSpace(probePath))
            {
                try
                {
                    File.Delete(probePath);
                }
                catch
                {
                    // Cleanup failure must not reject an otherwise writable directory.
                }
            }
        }
    }

    private static string ResolveStableApplicationRoot(string baseDirectory)
    {
        var trimmed = baseDirectory.TrimEnd(Path.DirectorySeparatorChar, Path.AltDirectorySeparatorChar);
        var directoryName = Path.GetFileName(trimmed);
        if (directoryName.Equals("current", StringComparison.OrdinalIgnoreCase) ||
            directoryName.StartsWith("app-", StringComparison.OrdinalIgnoreCase))
        {
            return Directory.GetParent(trimmed)?.FullName ?? trimmed;
        }

        return trimmed;
    }
}
