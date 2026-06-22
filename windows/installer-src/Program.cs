using System;
using System.IO;
using System.Linq;
using System.Windows.Forms;

internal static class Program
{
	private static readonly string DiagnosticLogPath = Path.Combine(Path.GetTempPath(), "VesperLauncherSetup.log");

	[STAThread]
	private static void Main(string[] args)
	{
		ApplicationConfiguration.Initialize();
		InstallerOptions installerOptions = InstallerOptions.Parse(args);
		if (installerOptions.RunUninstall)
		{
			HandleUninstall(installerOptions);
		}
		else if (installerOptions.RunSilently)
		{
			RunSilentInstall(installerOptions);
		}
		else
		{
			Application.Run(new InstallerForm(installerOptions));
		}
	}

	private static void HandleUninstall(InstallerOptions options)
	{
		try
		{
			if (options.RunSilently)
			{
				RunSilentUninstall(options);
			}
			else
			{
				Application.Run(new UninstallerForm(options));
			}
		}
		catch (Exception ex)
		{
			Environment.ExitCode = 1;
			if (!options.SuppressMessageBoxes)
			{
				MessageBox.Show(ex.Message, "Ошибка", MessageBoxButtons.OK, MessageBoxIcon.Hand);
			}
		}
	}

	private static void RunSilentInstall(InstallerOptions options)
	{
		try
		{
			AppendDiagnosticLog("Silent install started. Args: " + string.Join(" ", Environment.GetCommandLineArgs().Skip(1)));
			InstallResult installResult = InstallerOperations.Install(options, delegate
			{
			});
			AppendDiagnosticLog("Silent install finished. InstallDir=" + installResult.InstallDirectory + "; Exe=" + installResult.ExecutablePath);
			if (options.LaunchAfterInstall)
			{
				AppendDiagnosticLog("Launching installed app: " + installResult.ExecutablePath);
				InstallerOperations.TryLaunch(installResult.ExecutablePath);
			}
			Environment.ExitCode = 0;
		}
		catch (Exception ex)
		{
			AppendDiagnosticLog($"Silent install failed: {ex}");
			Environment.ExitCode = 1;
			if (!options.SuppressMessageBoxes)
			{
				MessageBox.Show(ex.Message, "Ошибка", MessageBoxButtons.OK, MessageBoxIcon.Hand);
			}
		}
	}

	private static void RunSilentUninstall(InstallerOptions options)
	{
		try
		{
			AppendDiagnosticLog("Silent uninstall started. Args: " + string.Join(" ", Environment.GetCommandLineArgs().Skip(1)));
			InstallerOperations.Uninstall(options, delegate
			{
			});
			AppendDiagnosticLog("Silent uninstall finished.");
			Environment.ExitCode = 0;
		}
		catch (Exception ex)
		{
			AppendDiagnosticLog($"Silent uninstall failed: {ex}");
			Environment.ExitCode = 1;
			if (!options.SuppressMessageBoxes)
			{
				MessageBox.Show(ex.Message, "Ошибка", MessageBoxButtons.OK, MessageBoxIcon.Hand);
			}
		}
	}

	private static void AppendDiagnosticLog(string message)
	{
		try
		{
			File.AppendAllText(DiagnosticLogPath, $"[{DateTime.Now:yyyy-MM-dd HH:mm:ss}] {message}{Environment.NewLine}");
		}
		catch
		{
		}
	}
}
