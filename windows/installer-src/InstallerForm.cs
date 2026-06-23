using System;
using System.ComponentModel;
using System.Drawing;
using System.Drawing.Drawing2D;
using System.Threading.Tasks;
using System.Windows.Forms;

internal sealed class InstallerForm : Form
{
    private readonly InstallerOptions _startupOptions;
    private readonly TextBox _pathTextBox = new();
    private readonly Button _browseButton = new();
    private readonly Button _installButton = new();
    private readonly Button _cancelButton = new();
    private readonly ProgressBar _progressBar = new();
    private readonly Label _statusLabel = new();
    private readonly CheckBox _launchAfterInstallCheckBox = new();

    public InstallerForm(InstallerOptions startupOptions)
    {
        _startupOptions = startupOptions;

        SuspendLayout();

        Text = "Vesper Launcher Setup";
        FormBorderStyle = FormBorderStyle.FixedSingle;
        MaximizeBox = false;
        MinimizeBox = true;
        StartPosition = FormStartPosition.CenterScreen;
        ClientSize = new Size(780, 500);
        AutoScaleMode = AutoScaleMode.Dpi;
        BackColor = Color.FromArgb(12, 18, 24);
        ForeColor = Color.White;
        Font = new Font("Segoe UI", 9f, FontStyle.Regular);

        var heroPanel = new GradientPanel
        {
            Left = 0,
            Top = 0,
            Width = 780,
            Height = 154,
            StartColor = Color.FromArgb(23, 35, 45),
            EndColor = Color.FromArgb(9, 16, 22)
        };

        var logo = new PictureBox
        {
            Left = 34,
            Top = 28,
            Width = 280,
            Height = 86,
            SizeMode = PictureBoxSizeMode.Zoom,
            BackColor = Color.Transparent,
            Image = InstallerAssets.LoadHeroImage() ?? InstallerAssets.LoadLogoImage()
        };

        var title = new Label
        {
            Left = 350,
            Top = 36,
            Width = 380,
            Height = 36,
            Text = "Установка Vesper Launcher",
            Font = new Font("Segoe UI Semibold", 19f, FontStyle.Bold),
            ForeColor = Color.White,
            BackColor = Color.Transparent
        };

        var subtitle = new Label
        {
            Left = 352,
            Top = 78,
            Width = 360,
            Height = 42,
            Text = "Выбери диск и папку. Обновления останутся безопасными через Velopack.",
            Font = new Font("Segoe UI", 10.5f, FontStyle.Regular),
            ForeColor = Color.FromArgb(210, 224, 235),
            BackColor = Color.Transparent
        };

        heroPanel.Controls.Add(logo);
        heroPanel.Controls.Add(title);
        heroPanel.Controls.Add(subtitle);

        var bodyPanel = new Panel
        {
            Left = 28,
            Top = 180,
            Width = 724,
            Height = 218,
            BackColor = Color.FromArgb(18, 27, 35)
        };

        var pathLabel = new Label
        {
            Left = 22,
            Top = 22,
            Width = 660,
            Height = 22,
            Text = "Папка установки",
            Font = new Font("Segoe UI Semibold", 11f, FontStyle.Bold),
            ForeColor = Color.White,
            BackColor = Color.Transparent
        };

        _pathTextBox.Left = 22;
        _pathTextBox.Top = 56;
        _pathTextBox.Width = 548;
        _pathTextBox.Height = 30;
        _pathTextBox.Text = InstallerPaths.ResolveInstallDirectory(_startupOptions.InstallDirectoryOverride);
        _pathTextBox.BackColor = Color.FromArgb(26, 38, 48);
        _pathTextBox.ForeColor = Color.White;
        _pathTextBox.BorderStyle = BorderStyle.FixedSingle;

        _browseButton.Left = 586;
        _browseButton.Top = 54;
        _browseButton.Width = 116;
        _browseButton.Height = 34;
        _browseButton.Text = "Обзор...";
        _browseButton.FlatStyle = FlatStyle.Flat;
        _browseButton.FlatAppearance.BorderColor = Color.FromArgb(86, 121, 146);
        _browseButton.BackColor = Color.FromArgb(30, 48, 62);
        _browseButton.ForeColor = Color.White;
        _browseButton.Click += (_, _) => BrowseFolder();

        var hint = new Label
        {
            Left = 22,
            Top = 102,
            Width = 660,
            Height = 42,
            Text = "Можно выбрать D:, E: или любую другую папку. Внутри будет запущен официальный Velopack setup с этим путем.",
            ForeColor = Color.FromArgb(196, 211, 222),
            BackColor = Color.Transparent
        };

        _launchAfterInstallCheckBox.Left = 22;
        _launchAfterInstallCheckBox.Top = 158;
        _launchAfterInstallCheckBox.Width = 330;
        _launchAfterInstallCheckBox.Height = 26;
        _launchAfterInstallCheckBox.Text = "Запустить лаунчер после установки";
        _launchAfterInstallCheckBox.Checked = _startupOptions.LaunchAfterInstall;
        _launchAfterInstallCheckBox.ForeColor = Color.White;
        _launchAfterInstallCheckBox.BackColor = Color.Transparent;

        bodyPanel.Controls.Add(pathLabel);
        bodyPanel.Controls.Add(_pathTextBox);
        bodyPanel.Controls.Add(_browseButton);
        bodyPanel.Controls.Add(hint);
        bodyPanel.Controls.Add(_launchAfterInstallCheckBox);

        _progressBar.Left = 28;
        _progressBar.Top = 418;
        _progressBar.Width = 724;
        _progressBar.Height = 16;
        _progressBar.Minimum = 0;
        _progressBar.Maximum = 100;
        _progressBar.Style = ProgressBarStyle.Continuous;

        _statusLabel.Left = 28;
        _statusLabel.Top = 444;
        _statusLabel.Width = 430;
        _statusLabel.Height = 24;
        _statusLabel.Text = "Готово к установке.";
        _statusLabel.ForeColor = Color.FromArgb(196, 211, 222);

        _installButton.Left = 526;
        _installButton.Top = 438;
        _installButton.Width = 106;
        _installButton.Height = 34;
        _installButton.Text = "Установить";
        _installButton.FlatStyle = FlatStyle.Flat;
        _installButton.FlatAppearance.BorderColor = Color.FromArgb(112, 177, 213);
        _installButton.BackColor = Color.FromArgb(42, 91, 119);
        _installButton.ForeColor = Color.White;
        _installButton.Click += async (_, _) => await InstallAsync();

        _cancelButton.Left = 646;
        _cancelButton.Top = 438;
        _cancelButton.Width = 106;
        _cancelButton.Height = 34;
        _cancelButton.Text = "Отмена";
        _cancelButton.FlatStyle = FlatStyle.Flat;
        _cancelButton.FlatAppearance.BorderColor = Color.FromArgb(78, 92, 102);
        _cancelButton.BackColor = Color.FromArgb(25, 34, 42);
        _cancelButton.ForeColor = Color.White;
        _cancelButton.Click += (_, _) => Close();

        AcceptButton = _installButton;
        CancelButton = _cancelButton;

        Controls.Add(heroPanel);
        Controls.Add(bodyPanel);
        Controls.Add(_progressBar);
        Controls.Add(_statusLabel);
        Controls.Add(_installButton);
        Controls.Add(_cancelButton);

        ResumeLayout(performLayout: false);
    }

    private void BrowseFolder()
    {
        using var folderBrowserDialog = new FolderBrowserDialog
        {
            Description = "Выбери папку для установки Vesper Launcher",
            UseDescriptionForTitle = true,
            SelectedPath = _pathTextBox.Text
        };

        if (folderBrowserDialog.ShowDialog(this) == DialogResult.OK)
        {
            _pathTextBox.Text = folderBrowserDialog.SelectedPath;
        }
    }

    private async Task InstallAsync()
    {
        var installPath = _pathTextBox.Text.Trim();
        if (string.IsNullOrWhiteSpace(installPath))
        {
            MessageBox.Show(this, "Укажи папку установки.", "Нет пути", MessageBoxButtons.OK, MessageBoxIcon.Exclamation);
            return;
        }

        SetUiEnabled(false);
        try
        {
            var options = _startupOptions with
            {
                InstallDirectoryOverride = installPath,
                LaunchAfterInstall = _launchAfterInstallCheckBox.Checked
            };

            var installResult = await Task.Run(() => InstallerOperations.Install(options, ReportProgress));
            _statusLabel.Text = "Установка завершена.";
            _progressBar.Value = 100;

            if (options.LaunchAfterInstall)
            {
                InstallerOperations.TryLaunch(installResult.ExecutablePath);
            }

            MessageBox.Show(this, "Vesper Launcher установлен.", "Готово", MessageBoxButtons.OK, MessageBoxIcon.Information);
            Close();
        }
        catch (Exception ex)
        {
            _statusLabel.Text = "Ошибка установки.";
            MessageBox.Show(this, ex.Message, "Ошибка", MessageBoxButtons.OK, MessageBoxIcon.Error);
        }
        finally
        {
            SetUiEnabled(true);
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
            Invoke(() => ReportProgress(progress));
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
        _launchAfterInstallCheckBox.Enabled = enabled;
        _installButton.Enabled = enabled;
    }

    private sealed class GradientPanel : Panel
    {
        [DesignerSerializationVisibility(DesignerSerializationVisibility.Hidden)]
        public Color StartColor { get; init; }

        [DesignerSerializationVisibility(DesignerSerializationVisibility.Hidden)]
        public Color EndColor { get; init; }

        protected override void OnPaint(PaintEventArgs e)
        {
            using var brush = new LinearGradientBrush(ClientRectangle, StartColor, EndColor, LinearGradientMode.Horizontal);
            e.Graphics.FillRectangle(brush, ClientRectangle);
            base.OnPaint(e);
        }
    }
}
