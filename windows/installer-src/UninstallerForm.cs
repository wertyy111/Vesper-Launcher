using System;
using System.Drawing;
using System.Threading.Tasks;
using System.Windows.Forms;

internal sealed class UninstallerForm : Form
{
	private readonly InstallerOptions _startupOptions;
	private readonly TextBox _pathTextBox = new TextBox();
	private readonly Label _statusLabel = new Label();
	private readonly ProgressBar _progressBar = new ProgressBar();
	private readonly CheckBox _removeDataCheckBox = new CheckBox();
	private readonly Button _uninstallButton = new Button();
	private readonly Button _cancelButton = new Button();

	public UninstallerForm(InstallerOptions startupOptions)
	{
		_startupOptions = startupOptions with
		{
			RunUninstall = true,
			CloseApplications = true
		};

		SuspendLayout();

		Text = "Vesper Launcher Uninstall";
		FormBorderStyle = FormBorderStyle.FixedDialog;
		MaximizeBox = false;
		MinimizeBox = false;
		StartPosition = FormStartPosition.CenterScreen;
		ClientSize = new Size(760, 450);
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
			Text = "Удаление Vesper Launcher",
			Font = new Font("Segoe UI Semibold", 18f, FontStyle.Bold),
			ForeColor = Color.FromArgb(28, 32, 39)
		};

		var headerSubtitle = new Label
		{
			Left = 84,
			Top = 48,
			AutoSize = true,
			Text = "Удаление программы с компьютера",
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
			Text = "Проверь путь удаления и выбери, нужно ли очищать локальные данные вместе с приложением.",
			ForeColor = Color.FromArgb(80, 86, 97)
		};

		var targetGroup = new GroupBox
		{
			Left = 20,
			Top = 148,
			Width = 708,
			Height = 92,
			Text = "Папка удаления"
		};

		_pathTextBox.Left = 16;
		_pathTextBox.Top = 36;
		_pathTextBox.Width = 674;
		_pathTextBox.ReadOnly = true;
		_pathTextBox.Text = InstallerPaths.ResolveInstallDirectoryForUninstall(_startupOptions.InstallDirectoryOverride);

		targetGroup.Controls.Add(_pathTextBox);

		var optionsGroup = new GroupBox
		{
			Left = 20,
			Top = 252,
			Width = 708,
			Height = 96,
			Text = "Параметры"
		};

		_removeDataCheckBox.Left = 16;
		_removeDataCheckBox.Top = 28;
		_removeDataCheckBox.Width = 520;
		_removeDataCheckBox.Text = "Удалить также кэш, настройки, сессию и локальные данные";
		_removeDataCheckBox.Checked = _startupOptions.RemoveUserData;

		var infoLabel = new Label
		{
			Left = 16,
			Top = 56,
			Width = 660,
			Height = 24,
			Text = "Если галочка снята, будет удалён только сам лаунчер, а пользовательские данные останутся.",
			ForeColor = Color.FromArgb(80, 86, 97)
		};

		optionsGroup.Controls.Add(_removeDataCheckBox);
		optionsGroup.Controls.Add(infoLabel);

		_progressBar.Left = 20;
		_progressBar.Top = 364;
		_progressBar.Width = 708;
		_progressBar.Height = 18;
		_progressBar.Minimum = 0;
		_progressBar.Maximum = 100;
		_progressBar.Style = ProgressBarStyle.Continuous;

		_statusLabel.Left = 20;
		_statusLabel.Top = 388;
		_statusLabel.Width = 430;
		_statusLabel.Height = 24;
		_statusLabel.Text = "Готово к удалению.";
		_statusLabel.ForeColor = Color.FromArgb(80, 86, 97);

		_uninstallButton.Left = 502;
		_uninstallButton.Top = 382;
		_uninstallButton.Width = 106;
		_uninstallButton.Height = 30;
		_uninstallButton.Text = "Удалить";
		_uninstallButton.Click += async delegate
		{
			await UninstallAsync();
		};

		_cancelButton.Left = 622;
		_cancelButton.Top = 382;
		_cancelButton.Width = 106;
		_cancelButton.Height = 30;
		_cancelButton.Text = "Отмена";
		_cancelButton.Click += delegate
		{
			Close();
		};

		AcceptButton = _uninstallButton;
		CancelButton = _cancelButton;

		Controls.Add(headerPanel);
		Controls.Add(introLabel);
		Controls.Add(targetGroup);
		Controls.Add(optionsGroup);
		Controls.Add(_progressBar);
		Controls.Add(_statusLabel);
		Controls.Add(_uninstallButton);
		Controls.Add(_cancelButton);

		ResumeLayout(performLayout: false);
	}

	private async Task UninstallAsync()
	{
		if (MessageBox.Show(this, "Удалить Vesper Launcher с этого компьютера?", "Подтверждение удаления", MessageBoxButtons.YesNo, MessageBoxIcon.Exclamation) != DialogResult.Yes)
		{
			return;
		}

		SetUiEnabled(enabled: false);
		try
		{
			InstallerOptions options = _startupOptions with
			{
				InstallDirectoryOverride = _pathTextBox.Text,
				RemoveUserData = _removeDataCheckBox.Checked,
				RunUninstall = true,
				CloseApplications = true
			};

			await Task.Run(() => InstallerOperations.Uninstall(options, ReportProgress));
			_statusLabel.Text = "Удаление завершено.";
			_progressBar.Value = 100;

			MessageBox.Show(this, "Лаунчер удалён.", "Готово", MessageBoxButtons.OK, MessageBoxIcon.Asterisk);
			Close();
		}
		catch (Exception ex)
		{
			_statusLabel.Text = "Ошибка удаления.";
			MessageBox.Show(this, ex.Message, "Ошибка", MessageBoxButtons.OK, MessageBoxIcon.Hand);
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
		_removeDataCheckBox.Enabled = enabled;
		_uninstallButton.Enabled = enabled;
		_cancelButton.Enabled = enabled;
	}
}
