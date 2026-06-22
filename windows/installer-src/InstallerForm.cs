using System;
using System.Drawing;
using System.Threading.Tasks;
using System.Windows.Forms;

internal sealed class InstallerForm : Form
{
	private readonly InstallerOptions _startupOptions;
	private readonly TextBox _pathTextBox = new TextBox();
	private readonly Button _browseButton = new Button();
	private readonly Button _installButton = new Button();
	private readonly Button _cancelButton = new Button();
	private readonly ProgressBar _progressBar = new ProgressBar();
	private readonly Label _statusLabel = new Label();
	private readonly CheckBox _desktopShortcutCheckBox = new CheckBox();
	private readonly CheckBox _startMenuShortcutCheckBox = new CheckBox();
	private readonly CheckBox _launchAfterInstallCheckBox = new CheckBox();

	public InstallerForm(InstallerOptions startupOptions)
	{
		_startupOptions = startupOptions;

		SuspendLayout();

		Text = "Vesper Launcher Setup";
		FormBorderStyle = FormBorderStyle.FixedDialog;
		MaximizeBox = false;
		MinimizeBox = false;
		StartPosition = FormStartPosition.CenterScreen;
		ClientSize = new Size(760, 470);
		AutoScaleMode = AutoScaleMode.Dpi;
		BackColor = SystemColors.Control;
		ForeColor = SystemColors.ControlText;
		Font = new Font("Segoe UI", 9f, FontStyle.Regular);

		var headerPanel = new Panel
		{
			Dock = DockStyle.Top,
			Height = 92,
			BackColor = Color.White
		};

		var headerDivider = new Panel
		{
			Dock = DockStyle.Bottom,
			Height = 1,
			BackColor = Color.FromArgb(220, 224, 230)
		};

		var headerLogo = new PictureBox
		{
			Left = 20,
			Top = 18,
			Width = 48,
			Height = 48,
			SizeMode = PictureBoxSizeMode.Zoom,
			BackColor = Color.Transparent,
			Image = InstallerAssets.LoadLogoImage() ?? InstallerAssets.LoadHeroImage()
		};

		var headerTitle = new Label
		{
			Left = 82,
			Top = 18,
			AutoSize = true,
			Text = "Vesper Launcher",
			Font = new Font("Segoe UI Semibold", 18f, FontStyle.Bold),
			ForeColor = Color.FromArgb(28, 32, 39)
		};

		var headerSubtitle = new Label
		{
			Left = 84,
			Top = 48,
			AutoSize = true,
			Text = "Установщик для Windows",
			Font = new Font("Segoe UI", 10f, FontStyle.Regular),
			ForeColor = Color.FromArgb(96, 104, 118)
		};

		headerPanel.Controls.Add(headerLogo);
		headerPanel.Controls.Add(headerTitle);
		headerPanel.Controls.Add(headerSubtitle);
		headerPanel.Controls.Add(headerDivider);

		var introLabel = new Label
		{
			Left = 20,
			Top = 108,
			Width = 708,
			Height = 36,
			Text = "Выбери папку установки и при необходимости настрой ярлыки. Всё остальное установщик сделает сам.",
			ForeColor = Color.FromArgb(80, 86, 97)
		};

		var installPathGroup = new GroupBox
		{
			Left = 20,
			Top = 148,
			Width = 708,
			Height = 92,
			Text = "Папка установки"
		};

		_pathTextBox.Left = 16;
		_pathTextBox.Top = 36;
		_pathTextBox.Width = 554;
		_pathTextBox.Text = InstallerPaths.ResolveInstallDirectory(_startupOptions.InstallDirectoryOverride);

		_browseButton.Left = 580;
		_browseButton.Top = 34;
		_browseButton.Width = 110;
		_browseButton.Height = 28;
		_browseButton.Text = "Обзор...";
		_browseButton.Click += delegate
		{
			BrowseFolder();
		};

		installPathGroup.Controls.Add(_pathTextBox);
		installPathGroup.Controls.Add(_browseButton);

		var optionsGroup = new GroupBox
		{
			Left = 20,
			Top = 252,
			Width = 708,
			Height = 118,
			Text = "Параметры"
		};

		_desktopShortcutCheckBox.Left = 16;
		_desktopShortcutCheckBox.Top = 28;
		_desktopShortcutCheckBox.Width = 260;
		_desktopShortcutCheckBox.Text = "Создать ярлык на рабочем столе";
		_desktopShortcutCheckBox.Checked = _startupOptions.CreateDesktopShortcut;

		_startMenuShortcutCheckBox.Left = 16;
		_startMenuShortcutCheckBox.Top = 54;
		_startMenuShortcutCheckBox.Width = 260;
		_startMenuShortcutCheckBox.Text = "Добавить в меню Пуск";
		_startMenuShortcutCheckBox.Checked = true;
		_startMenuShortcutCheckBox.Enabled = false;

		_launchAfterInstallCheckBox.Left = 16;
		_launchAfterInstallCheckBox.Top = 80;
		_launchAfterInstallCheckBox.Width = 320;
		_launchAfterInstallCheckBox.Text = "Запустить лаунчер после установки";
		_launchAfterInstallCheckBox.Checked = _startupOptions.LaunchAfterInstall;

		optionsGroup.Controls.Add(_desktopShortcutCheckBox);
		optionsGroup.Controls.Add(_startMenuShortcutCheckBox);
		optionsGroup.Controls.Add(_launchAfterInstallCheckBox);

		_progressBar.Left = 20;
		_progressBar.Top = 386;
		_progressBar.Width = 708;
		_progressBar.Height = 18;
		_progressBar.Minimum = 0;
		_progressBar.Maximum = 100;
		_progressBar.Style = ProgressBarStyle.Continuous;

		_statusLabel.Left = 20;
		_statusLabel.Top = 410;
		_statusLabel.Width = 430;
		_statusLabel.Height = 24;
		_statusLabel.Text = "Готово к установке.";
		_statusLabel.ForeColor = Color.FromArgb(80, 86, 97);

		_installButton.Left = 502;
		_installButton.Top = 404;
		_installButton.Width = 106;
		_installButton.Height = 30;
		_installButton.Text = "Установить";
		_installButton.Click += async delegate
		{
			await InstallAsync();
		};

		_cancelButton.Left = 622;
		_cancelButton.Top = 404;
		_cancelButton.Width = 106;
		_cancelButton.Height = 30;
		_cancelButton.Text = "Отмена";
		_cancelButton.Click += delegate
		{
			Close();
		};

		AcceptButton = _installButton;
		CancelButton = _cancelButton;

		Controls.Add(headerPanel);
		Controls.Add(introLabel);
		Controls.Add(installPathGroup);
		Controls.Add(optionsGroup);
		Controls.Add(_progressBar);
		Controls.Add(_statusLabel);
		Controls.Add(_installButton);
		Controls.Add(_cancelButton);

		UpdateInstallButtonState();
		ResumeLayout(performLayout: false);
	}

	private void BrowseFolder()
	{
		FolderBrowserDialog folderBrowserDialog = new FolderBrowserDialog
		{
			Description = "Выбери папку для установки",
			UseDescriptionForTitle = true,
			SelectedPath = _pathTextBox.Text
		};

		try
		{
			if (folderBrowserDialog.ShowDialog(this) == DialogResult.OK)
			{
				_pathTextBox.Text = folderBrowserDialog.SelectedPath;
			}
		}
		finally
		{
			((IDisposable)folderBrowserDialog).Dispose();
		}
	}

	private async Task InstallAsync()
	{
		string installPath = _pathTextBox.Text.Trim();
		if (string.IsNullOrWhiteSpace(installPath))
		{
			MessageBox.Show(this, "Укажи папку установки.", "Нет пути", MessageBoxButtons.OK, MessageBoxIcon.Exclamation);
			return;
		}

		SetUiEnabled(enabled: false);
		try
		{
			InstallerOptions options = _startupOptions with
			{
				InstallDirectoryOverride = installPath,
				CreateDesktopShortcut = _desktopShortcutCheckBox.Checked,
				LaunchAfterInstall = _launchAfterInstallCheckBox.Checked
			};

			InstallResult installResult = await Task.Run(() => InstallerOperations.Install(options, ReportProgress));
			_statusLabel.Text = "Установка завершена.";
			_progressBar.Value = 100;

			MessageBox.Show(this, "Лаунчер установлен.", "Готово", MessageBoxButtons.OK, MessageBoxIcon.Asterisk);
			if (options.LaunchAfterInstall)
			{
				InstallerOperations.TryLaunch(installResult.ExecutablePath);
			}
		}
		catch (Exception ex)
		{
			_statusLabel.Text = "Ошибка установки.";
			MessageBox.Show(this, ex.Message, "Ошибка", MessageBoxButtons.OK, MessageBoxIcon.Hand);
		}
		finally
		{
			SetUiEnabled(enabled: true);
		}
	}

	private void ReportProgress(InstallProgress progress)
	{
		if (IsDisposed)
		{
			return;
		}

		if (InvokeRequired)
		{
			Invoke(delegate
			{
				ReportProgress(progress);
			});
			return;
		}

		_statusLabel.Text = progress.Status;
		_progressBar.Value = Math.Max(_progressBar.Minimum, Math.Min(_progressBar.Maximum, progress.Percent));
	}

	private void SetUiEnabled(bool enabled)
	{
		_pathTextBox.Enabled = enabled;
		_browseButton.Enabled = enabled;
		_cancelButton.Enabled = enabled;
		_desktopShortcutCheckBox.Enabled = enabled;
		_launchAfterInstallCheckBox.Enabled = enabled;
		UpdateInstallButtonState(enabled);
	}

	private void UpdateInstallButtonState(bool uiEnabled = true)
	{
		_installButton.Enabled = uiEnabled;
	}
}
