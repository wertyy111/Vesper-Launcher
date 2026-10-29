package minecraft

import (
	"strings"
	"testing"

	"vesper-launcher/internal/auth"
)

func TestOnlineFixJvmArguments(t *testing.T) {
	ctx := LaunchContext{
		MinecraftDir: "/test/.minecraft",
		VersionID:    "1.20.1",
		NativesDir:   "/test/natives",
		Classpath:    "/test/cp.jar",
		Username:     "Steve",
		UUID:         "11111111-2222-3333-4444-555555555555",
		AccessToken:  "11111111222233334444555555555555",
		UserType:     "legacy",
		MemoryMb:     2048,
	}

	args := BuildJvmArguments(ctx)
	joined := strings.Join(args, " ")

	expectedFlags := []string{
		"-Dcom.mojang.minecraft.multiplayer=true",
		"-Dcom.mojang.minecraft.chat=true",
		"-XX:+IgnoreUnrecognizedVMOptions",
		"--add-exports=java.base/sun.security.util=ALL-UNNAMED",
		"--add-opens=java.base/java.net=ALL-UNNAMED",
		"--add-opens=java.base/java.lang=ALL-UNNAMED",
		"-Djava.net.preferIPv4Stack=true",
		"-Dfile.encoding=UTF-8",
	}

	for _, flag := range expectedFlags {
		if !strings.Contains(joined, flag) {
			t.Errorf("Expected JVM argument %q not found in args: %s", flag, joined)
		}
	}
}

func Test1165TLauncherMultiplayerFix(t *testing.T) {
	ctx := LaunchContext{
		MinecraftDir: "/test/.minecraft",
		VersionID:    "1.16.5",
		NativesDir:   "/test/natives",
		Classpath:    "/test/cp.jar",
		Username:     "Steve",
		UUID:         "11111111222233334444555555555555",
		AccessToken:  "11111111222233334444555555555555",
		UserType:     "mojang",
		MemoryMb:     2048,
	}

	args := BuildJvmArguments(ctx)
	joined := strings.Join(args, " ")

	expectedFlags := []string{
		"-Dcom.mojang.minecraft.multiplayer=true",
		"-Dcom.mojang.minecraft.onlineChat=true",
		"-Dminecraft.api.auth.host=https://nope.invalid",
		"-Dminecraft.api.account.host=https://nope.invalid",
		"-Dminecraft.api.session.host=https://nope.invalid",
		"-Dminecraft.api.services.host=https://nope.invalid",
	}

	for _, flag := range expectedFlags {
		if !strings.Contains(joined, flag) {
			t.Errorf("Expected 1.16.5 online fix flag %q not found in args: %s", flag, joined)
		}
	}

	replacements := getReplacements(ctx)
	if replacements["${clientid}"] != "offline-client" {
		t.Errorf("Expected clientid 'offline-client', got %s", replacements["${clientid}"])
	}
	if replacements["${auth_xuid}"] != "0" {
		t.Errorf("Expected auth_xuid '0', got %s", replacements["${auth_xuid}"])
	}
}

func TestOfflineProfileToken(t *testing.T) {
	prof := auth.CreateOfflineProfile("Player123")
	if prof.AccessToken == "00000000-0000-0000-0000-000000000000" {
		t.Errorf("AccessToken should not be dead dummy uuid, got %s", prof.AccessToken)
	}
	if len(prof.AccessToken) != 32 {
		t.Errorf("AccessToken should be 32-character hex, got %s (len %d)", prof.AccessToken, len(prof.AccessToken))
	}
	if prof.UserType != "legacy" {
		t.Errorf("UserType should be 'legacy', got %s", prof.UserType)
	}
}

func TestLegacyVersionDetection(t *testing.T) {
	if !isLegacyVersion("1.16.5") {
		t.Errorf("1.16.5 should be detected as legacy")
	}
	if !isLegacyVersion("ForgeOptiFine 1.16.5") {
		t.Errorf("ForgeOptiFine 1.16.5 should be detected as legacy")
	}
	if !isLegacyVersion("1.12.2") {
		t.Errorf("1.12.2 should be detected as legacy")
	}
	if !isLegacyVersion("1.7.10") {
		t.Errorf("1.7.10 should be detected as legacy")
	}
	if isLegacyVersion("1.20.1") {
		t.Errorf("1.20.1 should not be detected as legacy")
	}
	if isLegacyVersion("1.21.4") {
		t.Errorf("1.21.4 should not be detected as legacy")
	}
}

func TestMavenCoordinatesToPath(t *testing.T) {
	path, err := MavenCoordinatesToPath("org.ow2.asm:asm:9.7.1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := "org/ow2/asm/asm/9.7.1/asm-9.7.1.jar"
	if filepathToSlash(path) != expected {
		t.Errorf("expected %s, got %s", expected, filepathToSlash(path))
	}

	pathClassifier, err := MavenCoordinatesToPath("net.minecraftforge:forge:1.16.5-36.2.42:launcher")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expectedClassifier := "net/minecraftforge/forge/1.16.5-36.2.42/forge-1.16.5-36.2.42-launcher.jar"
	if filepathToSlash(pathClassifier) != expectedClassifier {
		t.Errorf("expected %s, got %s", expectedClassifier, filepathToSlash(pathClassifier))
	}
}

func filepathToSlash(p string) string {
	return strings.ReplaceAll(p, "\\", "/")
}
