using System;
using System.Collections.Generic;
using System.Diagnostics;
using System.IO;
using System.IO.Compression;
using System.Linq;
using System.Reflection;
using System.Runtime.InteropServices;
using Microsoft.Win32;

internal static class InstallerOperations
{
	private const int MoveFileDelayUntilReboot = 4;

	private static readonly HashSet<string> PayloadPreservedDirectories = new HashSet<string>(StringComparer.OrdinalIgnoreCase) { "GameData", ".launcher-data", ".launcher-temp-probe", ".launcher-updates" };

	private static readonly HashSet<string> EstimatedSizeExcludedDirectories = new HashSet<string>(StringComparer.OrdinalIgnoreCase) { "GameData", ".launcher-data", ".launcher-temp-probe", ".launcher-updates" };

	private static readonly HashSet<string> EstimatedSizeExcludedFiles = new HashSet<string>(StringComparer.OrdinalIgnoreCase) { "installer.log", "launcher-errors.log", "_last_java_stdout.log", "_last_java_stderr.log" };

	[DllImport("kernel32.dll", CharSet = CharSet.Unicode, SetLastError = true)]
	private static extern bool MoveFileEx(string? existingFileName, string? newFileName, int flags);

	public static InstallResult Install(InstallerOptions options, Action<InstallProgress> reportProgress)
	{
		string text = InstallerPaths.ResolveInstallDirectory(options.InstallDirectoryOverride);
		Report(reportProgress, "Подготовка папок...", 2);
		EnsureRuntimeDirectories(text);
		if (options.CloseApplications)
		{
			Report(reportProgress, "Закрываю запущенный лаунчер...", 6);
			CloseRunningLauncherProcesses(text);
		}
		Report(reportProgress, "Распаковка...", 8);
		ExtractPayload(text, delegate(int progress)
		{
			Report(reportProgress, "Распаковка...", progress);
		});
		InstallerAssets.WriteLicenseFile(text);
		ValidateInstalledLayout(text);
		string text2 = Path.Combine(text, "MinecraftCheatLauncher.exe");
		Report(reportProgress, "Подготавливаю деинсталлятор...", 96);
		string text3 = PrepareInstalledUninstaller(text);
		if (options.CreateDesktopShortcut)
		{
			CreateShortcut(Environment.GetFolderPath(Environment.SpecialFolder.DesktopDirectory), text2, "Vesper Launcher");
		}
		else
		{
			DeleteFileIfExists(Path.Combine(Environment.GetFolderPath(Environment.SpecialFolder.DesktopDirectory), "Vesper Launcher.lnk"));
		}
		string programsFolder = GetProgramsFolder();
		DeleteLegacyUninstallShortcuts(programsFolder);
		CreateShortcut(programsFolder, text2, "Vesper Launcher");
		CreateShortcut(programsFolder, text3, "Uninstall Vesper Launcher", "/UNINSTALL", text2);
		Report(reportProgress, "Регистрирую приложение в Windows...", 98);
		RegisterInstallation(text, text2, text3);
		Report(reportProgress, "Установка завершена.", 100);
		return new InstallResult(text, text2, text3);
	}

	public static UninstallResult Uninstall(InstallerOptions options, Action<InstallProgress> reportProgress)
	{
		string text = InstallerPaths.ResolveInstallDirectoryForUninstall(options.InstallDirectoryOverride);
		bool flag = CurrentProcessLivesInsideInstallDirectory(text);
		Report(reportProgress, "Подготовка к удалению...", 4);
		CloseRunningLauncherProcesses(text);
		Report(reportProgress, "Удаляю ярлыки...", 18);
		RemoveShortcuts();
		Report(reportProgress, "Удаляю запись о приложении...", 28);
		RemoveUninstallRegistration();
		Report(reportProgress, "Удаляю файлы лаунчера...", 38);
		if (flag)
		{
			DeleteInstallationDirectoryContentsExceptCurrentRuntime(text);
			ScheduleDeferredInstallDirectoryCleanup(text);
		}
		else
		{
			DeleteInstallationDirectory(text);
		}
		if (options.RemoveUserData)
		{
			Report(reportProgress, "Удаляю данные лаунчера...", 84);
			DeleteLauncherData();
		}
		else
		{
			Report(reportProgress, "Пользовательские данные сохранены.", 84);
		}
		RemoveEmptyStartMenuFolder();
		Report(reportProgress, "Удаление завершено.", 100);
		return new UninstallResult(text, options.RemoveUserData);
	}

	public static void TryLaunch(string exePath)
	{
		try
		{
			Process.Start(new ProcessStartInfo
			{
				FileName = exePath,
				UseShellExecute = true
			});
		}
		catch
		{
		}
	}

	private static void RegisterInstallation(string installDir, string exePath, string uninstallerPath)
	{
		using RegistryKey registryKey = Registry.CurrentUser.CreateSubKey("Software\\Microsoft\\Windows\\CurrentVersion\\Uninstall\\VesperLauncher", writable: true);
		if (registryKey != null)
		{
			string value = TryGetFileVersion(exePath) ?? TryGetInstallerVersion() ?? "1.0.0";
			registryKey.SetValue("DisplayName", "Vesper Launcher");
			registryKey.SetValue("DisplayVersion", value);
			registryKey.SetValue("Publisher", "Vesper Launcher");
			registryKey.SetValue("InstallLocation", installDir);
			registryKey.SetValue("DisplayIcon", exePath);
			registryKey.SetValue("UninstallString", "\"" + uninstallerPath + "\" /UNINSTALL");
			registryKey.SetValue("QuietUninstallString", "\"" + uninstallerPath + "\" /UNINSTALL /VERYSILENT /SUPPRESSMSGBOXES /CLOSEAPPLICATIONS");
			registryKey.SetValue("NoModify", 1, RegistryValueKind.DWord);
			registryKey.SetValue("NoRepair", 1, RegistryValueKind.DWord);
			registryKey.SetValue("InstallDate", DateTime.Now.ToString("yyyyMMdd"));
			registryKey.SetValue("EstimatedSize", CalculateInstalledApplicationSizeInKilobytes(installDir), RegistryValueKind.DWord);
			registryKey.SetValue("Readme", Path.Combine(installDir, "LICENSE.txt"));
			registryKey.SetValue("Comments", "Условия использования доступны в файле LICENSE.txt в папке приложения.");
		}
	}

	private static void RemoveUninstallRegistration()
	{
		try
		{
			Registry.CurrentUser.DeleteSubKeyTree("Software\\Microsoft\\Windows\\CurrentVersion\\Uninstall\\VesperLauncher", throwOnMissingSubKey: false);
		}
		catch
		{
		}
	}

	private static string? TryGetFileVersion(string filePath)
	{
		try
		{
			if (!File.Exists(filePath))
			{
				return null;
			}
			return FileVersionInfo.GetVersionInfo(filePath).ProductVersion;
		}
		catch
		{
			return null;
		}
	}

	private static string? TryGetInstallerVersion()
	{
		try
		{
			return Assembly.GetExecutingAssembly().GetName().Version?.ToString(3);
		}
		catch
		{
			return null;
		}
	}

	private static int CalculateInstalledApplicationSizeInKilobytes(string directoryPath)
	{
		try
		{
			if (!Directory.Exists(directoryPath))
			{
				return 0;
			}
			long num = 0L;
			foreach (string item in Directory.EnumerateFiles(directoryPath, "*", SearchOption.AllDirectories))
			{
				try
				{
					if (!ShouldExcludeFromEstimatedSize(directoryPath, item))
					{
						num += new FileInfo(item).Length;
					}
				}
				catch
				{
				}
			}
			return (int)Math.Clamp(num / 1024, 0L, 2147483647L);
		}
		catch
		{
			return 0;
		}
	}

	private static bool ShouldExcludeFromEstimatedSize(string rootDirectory, string filePath)
	{
		string relativePath = Path.GetRelativePath(rootDirectory, filePath);
		if (string.IsNullOrWhiteSpace(relativePath) || relativePath.StartsWith("..", StringComparison.Ordinal))
		{
			return false;
		}
		string[] array = relativePath.Split(new char[2]
		{
			Path.DirectorySeparatorChar,
			Path.AltDirectorySeparatorChar
		}, StringSplitOptions.RemoveEmptyEntries);
		if (array.Length == 0)
		{
			return false;
		}
		if (!EstimatedSizeExcludedDirectories.Contains(array[0]))
		{
			if (array.Length == 1)
			{
				return EstimatedSizeExcludedFiles.Contains(array[0]);
			}
			return false;
		}
		return true;
	}

	private static string PrepareInstalledUninstaller(string installDir)
	{
		RemoveLegacyUninstallArtifacts(installDir);
		string text = ProcessPathUtilities.TryGetLaunchedExecutablePath();
		if (string.IsNullOrWhiteSpace(text) || !File.Exists(text))
		{
			throw new FileNotFoundException("РќРµ СѓРґР°Р»РѕСЃСЊ РЅР°Р№С‚Рё С‚РµРєСѓС‰РёР№ setup.exe.", text);
		}
		string text2 = Path.Combine(installDir, "uninstall.exe");
		File.Copy(text, text2, overwrite: true);
		CopyInstallerCompanionFile(text, ".deps.json", installDir);
		CopyInstallerCompanionFile(text, ".runtimeconfig.json", installDir);
		string text3 = Path.ChangeExtension(text, ".dll");
		if (File.Exists(text3))
		{
			File.Copy(text3, Path.Combine(installDir, Path.GetFileName(text3)), overwrite: true);
		}
		return text2;
	}

	private static void RemoveLegacyUninstallArtifacts(string installDir)
	{
		DeleteFileIfExists(Path.Combine(installDir, "uninstall.ps1"));
		DeleteFileIfExists(Path.Combine(installDir, "uninstall.cmd"));
		DeleteFileIfExists(Path.Combine(installDir, "Installer.exe"));
		DeleteFileIfExists(Path.Combine(installDir, "uninstall.exe"));
	}

	private static void CopyInstallerCompanionFile(string currentInstallerPath, string extension, string installDir)
	{
		string text = Path.ChangeExtension(currentInstallerPath, extension);
		if (File.Exists(text))
		{
			File.Copy(text, Path.Combine(installDir, Path.GetFileName(text)), overwrite: true);
		}
	}

	private static void DeleteInstallationDirectory(string installDir)
	{
		if (!string.IsNullOrWhiteSpace(installDir) && Directory.Exists(installDir))
		{
			ClearDirectoryAttributes(installDir);
			Directory.Delete(installDir, recursive: true);
		}
	}

	private static void DeleteInstallationDirectoryContentsExceptCurrentRuntime(string installDir)
	{
		if (string.IsNullOrWhiteSpace(installDir) || !Directory.Exists(installDir))
		{
			return;
		}
		HashSet<string> hashSet = new HashSet<string>(StringComparer.OrdinalIgnoreCase);
		string text = ProcessPathUtilities.TryGetLaunchedExecutablePath();
		if (!string.IsNullOrWhiteSpace(text))
		{
			hashSet.Add(Path.GetFullPath(text));
		}
		string name = Assembly.GetExecutingAssembly().GetName().Name;
		if (!string.IsNullOrWhiteSpace(name))
		{
			string[] array = new string[3] { ".dll", ".deps.json", ".runtimeconfig.json" };
			foreach (string text2 in array)
			{
				string path = Path.Combine(installDir, name + text2);
				if (File.Exists(path))
				{
					hashSet.Add(Path.GetFullPath(path));
				}
			}
		}
		foreach (string item in Directory.EnumerateFiles(installDir, "*", SearchOption.AllDirectories))
		{
			try
			{
				string fullPath = Path.GetFullPath(item);
				if (!hashSet.Contains(fullPath))
				{
					File.SetAttributes(fullPath, FileAttributes.Normal);
					File.Delete(fullPath);
				}
			}
			catch
			{
			}
		}
		foreach (string item2 in from text3 in Directory.EnumerateDirectories(installDir, "*", SearchOption.AllDirectories)
			orderby text3.Length descending
			select text3)
		{
			try
			{
				if (!Directory.EnumerateFileSystemEntries(item2).Any())
				{
					Directory.Delete(item2, recursive: false);
				}
			}
			catch
			{
			}
		}
	}

	private static bool CurrentProcessLivesInsideInstallDirectory(string installDir)
	{
		string text = ProcessPathUtilities.TryGetLaunchedExecutablePath();
		if (string.IsNullOrWhiteSpace(text))
		{
			return false;
		}
		string directoryName = Path.GetDirectoryName(text);
		if (string.IsNullOrWhiteSpace(directoryName))
		{
			return false;
		}
		return InstallerPaths.PathsEqual(directoryName, installDir);
	}

	private static void ScheduleDeferredInstallDirectoryCleanup(string installDir)
	{
		if (string.IsNullOrWhiteSpace(installDir))
		{
			return;
		}
		List<string> list = new List<string>();
		string text = ProcessPathUtilities.TryGetLaunchedExecutablePath();
		if (!string.IsNullOrWhiteSpace(text))
		{
			list.Add(text);
		}
		string name = Assembly.GetExecutingAssembly().GetName().Name;
		if (!string.IsNullOrWhiteSpace(name))
		{
			string[] array = new string[3] { ".dll", ".deps.json", ".runtimeconfig.json" };
			foreach (string text2 in array)
			{
				string text3 = Path.Combine(installDir, name + text2);
				if (File.Exists(text3))
				{
					list.Add(text3);
				}
			}
		}
		foreach (string item in list.Distinct<string>(StringComparer.OrdinalIgnoreCase))
		{
			TryScheduleDeleteOnReboot(item);
		}
		foreach (string item2 in from path in Directory.EnumerateDirectories(installDir, "*", SearchOption.AllDirectories)
			orderby path.Length descending
			select path)
		{
			TryScheduleDeleteOnReboot(item2);
		}
		TryScheduleDeleteOnReboot(installDir);
	}

	private static void TryScheduleDeleteOnReboot(string path)
	{
		try
		{
			if (!string.IsNullOrWhiteSpace(path))
			{
				string fullPath = Path.GetFullPath(path);
				if (File.Exists(fullPath) || Directory.Exists(fullPath))
				{
					MoveFileEx(fullPath, null, 4);
				}
			}
		}
		catch
		{
		}
	}

	private static void DeleteLauncherData()
	{
		DeleteDirectoryIfExists(Path.Combine(Environment.GetFolderPath(Environment.SpecialFolder.ApplicationData), "VesperLauncher"));
		DeleteDirectoryIfExists(Path.Combine(Environment.GetFolderPath(Environment.SpecialFolder.LocalApplicationData), "Vesper Launcher"));
	}

	private static void DeleteDirectoryIfExists(string directoryPath)
	{
		if (Directory.Exists(directoryPath))
		{
			ClearDirectoryAttributes(directoryPath);
			Directory.Delete(directoryPath, recursive: true);
		}
	}

	private static void ClearDirectoryAttributes(string rootDirectory)
	{
		foreach (string item in Directory.EnumerateFiles(rootDirectory, "*", SearchOption.AllDirectories))
		{
			try
			{
				File.SetAttributes(item, FileAttributes.Normal);
			}
			catch
			{
			}
		}
	}

	private static string GetProgramsFolder()
	{
		return Path.Combine(Environment.GetFolderPath(Environment.SpecialFolder.StartMenu), "Programs", "Vesper Launcher");
	}

	private static void RemoveShortcuts()
	{
		DeleteFileIfExists(Path.Combine(Environment.GetFolderPath(Environment.SpecialFolder.DesktopDirectory), "Vesper Launcher.lnk"));
		string programsFolder = GetProgramsFolder();
		DeleteFileIfExists(Path.Combine(programsFolder, "Vesper Launcher.lnk"));
		DeleteFileIfExists(Path.Combine(programsFolder, "Uninstall Vesper Launcher.lnk"));
		DeleteLegacyUninstallShortcuts(programsFolder);
	}

	private static void RemoveEmptyStartMenuFolder()
	{
		try
		{
			string programsFolder = GetProgramsFolder();
			if (Directory.Exists(programsFolder) && !Directory.EnumerateFileSystemEntries(programsFolder).Any())
			{
				Directory.Delete(programsFolder, recursive: false);
			}
		}
		catch
		{
		}
	}

	private static void DeleteLegacyUninstallShortcuts(string programsFolder)
	{
		string[] legacyUninstallShortcutNames = InstallerConstants.LegacyUninstallShortcutNames;
		foreach (string text in legacyUninstallShortcutNames)
		{
			if (!string.Equals(text, "Uninstall Vesper Launcher", StringComparison.Ordinal))
			{
				DeleteFileIfExists(Path.Combine(programsFolder, text + ".lnk"));
			}
		}
	}

	private static void DeleteFileIfExists(string filePath)
	{
		try
		{
			if (File.Exists(filePath))
			{
				File.SetAttributes(filePath, FileAttributes.Normal);
				File.Delete(filePath);
			}
		}
		catch
		{
		}
	}

	private static void Report(Action<InstallProgress> reportProgress, string status, int percent)
	{
		reportProgress(new InstallProgress(status, percent));
	}

	private static void EnsureRuntimeDirectories(string installDir)
	{
		Directory.CreateDirectory(installDir);
		string text = Path.Combine(Environment.GetFolderPath(Environment.SpecialFolder.ApplicationData), "VesperLauncher");
		Directory.CreateDirectory(text);
		Directory.CreateDirectory(Path.Combine(text, "authlib-injector"));
		Directory.CreateDirectory(Path.Combine(text, "cache"));
		Directory.CreateDirectory(Path.Combine(text, "skins"));
		Directory.CreateDirectory(Path.Combine(Environment.GetFolderPath(Environment.SpecialFolder.LocalApplicationData), "Vesper Launcher"));
	}

	private static void ValidateInstalledLayout(string installDir)
	{
		string[] array = new string[3] { "Assets", "BundledLibraries", "BundledVersions" };
		foreach (string text in array)
		{
			if (!Directory.Exists(Path.Combine(installDir, text)))
			{
				throw new DirectoryNotFoundException("Не найдена обязательная папка: " + text);
			}
		}
		array = new string[2] { "MinecraftCheatLauncher.exe", "MinecraftCheatLauncher.dll" };
		foreach (string text2 in array)
		{
			string text3 = Path.Combine(installDir, text2);
			if (!File.Exists(text3))
			{
				throw new FileNotFoundException("Не найден обязательный файл: " + text2, text3);
			}
		}
	}

	private static void ExtractPayload(string installDir, Action<int> reportPercent)
	{
		Directory.CreateDirectory(installDir);
		Assembly executingAssembly = Assembly.GetExecutingAssembly();
		string text = executingAssembly.GetManifestResourceNames().FirstOrDefault((string name) => name.EndsWith("payload.zip", StringComparison.OrdinalIgnoreCase));
		if (text == null)
		{
			throw new InvalidOperationException("В установщике не найден payload.zip.");
		}
		using Stream stream = executingAssembly.GetManifestResourceStream(text);
		if (stream == null)
		{
			throw new InvalidOperationException("Не удалось открыть ресурс payload.zip.");
		}
		using ZipArchive zipArchive = new ZipArchive(stream, ZipArchiveMode.Read, leaveOpen: false);
		ResetPayloadDirectories(installDir, zipArchive.Entries);
		List<ZipArchiveEntry> list = zipArchive.Entries.Where((ZipArchiveEntry entry) => !string.IsNullOrEmpty(entry.Name)).ToList();
		int num = Math.Max(1, list.Count);
		int num2 = 0;
		foreach (ZipArchiveEntry entry in zipArchive.Entries)
		{
			string text2 = Path.Combine(installDir, entry.FullName);
			string directoryName = Path.GetDirectoryName(text2);
			if (!string.IsNullOrWhiteSpace(directoryName))
			{
				Directory.CreateDirectory(directoryName);
			}
			if (!string.IsNullOrEmpty(entry.Name))
			{
				entry.ExtractToFile(text2, overwrite: true);
				num2++;
				int val = 8 + (int)Math.Round((double)num2 / (double)num * 88.0);
				reportPercent(Math.Min(95, val));
			}
		}
	}

	private static void ResetPayloadDirectories(string installDir, IEnumerable<ZipArchiveEntry> entries)
	{
		foreach (string item in (from entry in entries
			select TryGetTopLevelDirectoryName(entry.FullName) into name
			where !string.IsNullOrWhiteSpace(name)
			select name).Distinct<string>(StringComparer.OrdinalIgnoreCase))
		{
			if (item != null && !PayloadPreservedDirectories.Contains(item))
			{
				DeleteDirectoryIfExists(Path.Combine(installDir, item));
			}
		}
	}

	private static string? TryGetTopLevelDirectoryName(string entryPath)
	{
		if (string.IsNullOrWhiteSpace(entryPath))
		{
			return null;
		}
		string text = entryPath.Replace('\\', '/').Trim('/');
		int num = text.IndexOf('/');
		if (num > 0)
		{
			return text.Substring(0, num);
		}
		return null;
	}

	private static void CloseRunningLauncherProcesses(string installDir)
	{
		string? fileNameWithoutExtension = Path.GetFileNameWithoutExtension("MinecraftCheatLauncher.exe");
		string fullPath = Path.GetFullPath(installDir);
		Process[] processesByName = Process.GetProcessesByName(fileNameWithoutExtension);
		foreach (Process process in processesByName)
		{
			try
			{
				if (process.Id != Environment.ProcessId)
				{
					string text = TryGetProcessPath(process);
					if (string.IsNullOrWhiteSpace(text))
					{
						goto IL_0060;
					}
					string directoryName = Path.GetDirectoryName(text);
					if (string.IsNullOrWhiteSpace(directoryName) || InstallerPaths.PathsEqual(directoryName, fullPath))
					{
						goto IL_0060;
					}
				}
				goto end_IL_0022;
				IL_0060:
				if (!process.HasExited && process.CloseMainWindow())
				{
					process.WaitForExit(8000);
				}
				if (!process.HasExited)
				{
					process.Kill(entireProcessTree: true);
					process.WaitForExit(5000);
				}
				end_IL_0022:;
			}
			catch
			{
			}
		}
	}

	private static string? TryGetProcessPath(Process process)
	{
		try
		{
			return process.MainModule?.FileName;
		}
		catch
		{
			return null;
		}
	}

	private static void CreateShortcut(string folder, string targetPath, string shortcutName, string? arguments = null, string? iconLocation = null)
	{
		Directory.CreateDirectory(folder);
		string text = Path.Combine(folder, shortcutName + ".lnk");
		Type typeFromProgID = Type.GetTypeFromProgID("WScript.Shell");
		if ((object)typeFromProgID != null)
		{
			dynamic val = Activator.CreateInstance(typeFromProgID);
			dynamic val2 = val.CreateShortcut(text);
			val2.TargetPath = targetPath;
			val2.Arguments = arguments ?? string.Empty;
			val2.WorkingDirectory = Path.GetDirectoryName(targetPath);
			val2.IconLocation = (string.IsNullOrWhiteSpace(iconLocation) ? targetPath : iconLocation);
			val2.Save();
		}
	}
}
