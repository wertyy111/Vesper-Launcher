using System;
using System.Drawing;
using System.Drawing.Imaging;
using System.IO;
using System.Linq;
using System.Reflection;
using System.Text;

internal static class InstallerAssets
{
	public static Image? LoadHeroImage()
	{
		try
		{
			Assembly executingAssembly = Assembly.GetExecutingAssembly();
			string text = executingAssembly.GetManifestResourceNames().FirstOrDefault((string name) => name.EndsWith("vesper-launcher-wordmark.png", StringComparison.OrdinalIgnoreCase) || name.EndsWith("vesper-launcher-wordmark.jpg", StringComparison.OrdinalIgnoreCase) || name.EndsWith("vesper-launcher-wordmark.jpeg", StringComparison.OrdinalIgnoreCase));
			if (text == null)
			{
				return null;
			}
			using Stream stream = executingAssembly.GetManifestResourceStream(text);
			if (stream == null)
			{
				return null;
			}
			return new Bitmap(stream);
		}
		catch
		{
			return null;
		}
	}

	public static string LoadLicenseText()
	{
		try
		{
			Assembly executingAssembly = Assembly.GetExecutingAssembly();
			string text = executingAssembly.GetManifestResourceNames().FirstOrDefault((string name) => name.EndsWith("installer-license.txt", StringComparison.OrdinalIgnoreCase) || name.EndsWith("license.txt", StringComparison.OrdinalIgnoreCase));
			if (text == null)
			{
				return GetBundledFallbackLicenseText();
			}
			using Stream stream = executingAssembly.GetManifestResourceStream(text);
			if (stream == null)
			{
				return GetBundledFallbackLicenseText();
			}
			using StreamReader streamReader = new StreamReader(stream);
			return streamReader.ReadToEnd();
		}
		catch
		{
			return GetBundledFallbackLicenseText();
		}
	}

	public static void WriteLicenseFile(string installDir)
	{
		Directory.CreateDirectory(installDir);
		File.WriteAllText(Path.Combine(installDir, "LICENSE.txt"), LoadLicenseText(), new UTF8Encoding(encoderShouldEmitUTF8Identifier: true));
	}

	public static Image? LoadLogoImage()
	{
		try
		{
			Assembly executingAssembly = Assembly.GetExecutingAssembly();
			string text = executingAssembly.GetManifestResourceNames().FirstOrDefault((string name) => name.EndsWith("vesper-logo.png", StringComparison.OrdinalIgnoreCase) || name.EndsWith("vesper-logo.jpg", StringComparison.OrdinalIgnoreCase) || name.EndsWith("vesper-logo.jpeg", StringComparison.OrdinalIgnoreCase));
			if (text == null)
			{
				return null;
			}
			using Stream stream = executingAssembly.GetManifestResourceStream(text);
			if (stream == null)
			{
				return null;
			}
			Bitmap bitmap = new Bitmap(stream);
			if (bitmap.PixelFormat == PixelFormat.Format32bppArgb || bitmap.PixelFormat == PixelFormat.Format64bppArgb)
			{
				return bitmap;
			}
			return RemoveBackgroundByCornerColor(bitmap);
		}
		catch
		{
			return null;
		}
	}

	private static Bitmap RemoveBackgroundByCornerColor(Bitmap source)
	{
		Bitmap bitmap = new Bitmap(source.Width, source.Height, PixelFormat.Format32bppArgb);
		Color pixel = source.GetPixel(0, 0);
		for (int i = 0; i < source.Height; i++)
		{
			for (int j = 0; j < source.Width; j++)
			{
				Color pixel2 = source.GetPixel(j, i);
				if (Math.Abs(pixel2.R - pixel.R) + Math.Abs(pixel2.G - pixel.G) + Math.Abs(pixel2.B - pixel.B) <= 95)
				{
					bitmap.SetPixel(j, i, Color.FromArgb(0, pixel2.R, pixel2.G, pixel2.B));
				}
				else
				{
					bitmap.SetPixel(j, i, Color.FromArgb(255, pixel2.R, pixel2.G, pixel2.B));
				}
			}
		}
		source.Dispose();
		return bitmap;
	}

	private static string GetBundledFallbackLicenseText()
	{
		return "Лицензионное соглашение Vesper Launcher" + Environment.NewLine + Environment.NewLine + "Устанавливая или используя Vesper Launcher, ты принимаешь условия лицензии, приложенной к этому приложению.";
	}
}
