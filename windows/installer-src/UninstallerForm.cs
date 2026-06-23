using System;
using System.Drawing;
using System.Threading.Tasks;
using System.Windows.Forms;

internal sealed class UninstallerForm : Form
{
    private readonly InstallerOptions _startupOptions;
    private readonly Label _statusLabel = new();
    private readonly ProgressBar _progressBar = new();
    private readonly Button _uninstallButton = new();
    private readonly Button _cancelButton = new();

    public UninstallerForm(InstallerOptions startupOptions)
    {
        _startupOptions = startupOptions with
        {
            RunUninstall = true,
            CloseApplications = true
        };

        SuspendLayout();

        Text = "Vesper Launcher Uninstall";
        FormBorderStyle = FormBorderStyle.FixedSingle;
        MaximizeBox = false;
        StartPosition = FormStartPosition.CenterScreen;
        ClientSize = new Size(560, 240);
        AutoScaleMode = AutoScaleMode.Dpi;
        BackColor = Color.FromArgb(12, 18, 24);
        ForeColor = Color.White;
        Font = new Font("Segoe UI", 9f, FontStyle.Regular);

        var title = new Label
        {
            Left = 28,
            Top = 28,
            Width = 500,
            Height = 34,
            Text = "Удаление Vesper Launcher",
            Font = new Font("Segoe UI Semibold", 17f, FontStyle.Bold),
            ForeColor = Color.White
        };

        var description = new Label
        {
            Left = 30,
            Top = 76,
            Width = 500,
            Height = 44,
            Text = "Будет запущено штатное удаление Velopack. Пользовательские данные Minecraft не удаляются автоматически.",
            ForeColor = Color.FromArgb(196, 211, 222)
        };

        _progressBar.Left = 30;
        _progressBar.Top = 136;
        _progressBar.Width = 500;
        _progressBar.Height = 16;

        _statusLabel.Left = 30;
        _statusLabel.Top = 162;
        _statusLabel.Width = 310;
        _statusLabel.Height = 22;
        _statusLabel.Text = "Готово к удалению.";
        _statusLabel.ForeColor = Color.FromArgb(196, 211, 222);

        _uninstallButton.Left = 312;
        _uninstallButton.Top = 188;
        _uninstallButton.Width = 104;
        _uninstallButton.Height = 32;
        _uninstallButton.Text = "Удалить";
        _uninstallButton.FlatStyle = FlatStyle.Flat;
        _uninstallButton.BackColor = Color.FromArgb(42, 91, 119);
        _uninstallButton.ForeColor = Color.White;
        _uninstallButton.Click += async (_, _) => await UninstallAsync();

        _cancelButton.Left = 426;
        _cancelButton.Top = 188;
        _cancelButton.Width = 104;
        _cancelButton.Height = 32;
        _cancelButton.Text = "Отмена";
        _cancelButton.FlatStyle = FlatStyle.Flat;
        _cancelButton.BackColor = Color.FromArgb(25, 34, 42);
        _cancelButton.ForeColor = Color.White;
        _cancelButton.Click += (_, _) => Close();

        Controls.Add(title);
        Controls.Add(description);
        Controls.Add(_progressBar);
        Controls.Add(_statusLabel);
        Controls.Add(_uninstallButton);
        Controls.Add(_cancelButton);

        ResumeLayout(performLayout: false);
    }

    private async Task UninstallAsync()
    {
        if (MessageBox.Show(this, "Удалить Vesper Launcher?", "Подтверждение", MessageBoxButtons.YesNo, MessageBoxIcon.Question) != DialogResult.Yes)
        {
            return;
        }

        SetUiEnabled(false);
        try
        {
            await Task.Run(() => InstallerOperations.Uninstall(_startupOptions, ReportProgress));
            _statusLabel.Text = "Удаление запущено.";
            _progressBar.Value = 100;
            Close();
        }
        catch (Exception ex)
        {
            _statusLabel.Text = "Ошибка удаления.";
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
        _uninstallButton.Enabled = enabled;
        _cancelButton.Enabled = enabled;
    }
}
