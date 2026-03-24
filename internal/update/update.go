package update

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/donnycrash/clasp/internal/config"
)

const (
	repo          = "donnycrash/clasp"
	cacheTTL      = 24 * time.Hour
	cacheFile     = "update-check.json"
	apiTimeout    = 5 * time.Second
	fetchTimeout  = 60 * time.Second
	maxBinarySize = 100 << 20 // 100 MB
)

var allowedDownloadHosts = []string{
	"github.com",
	"objects.githubusercontent.com",
}

type Release struct {
	TagName string `json:"tag_name"`
}

type cachedCheck struct {
	LatestVersion string    `json:"latest_version"`
	CheckedAt     time.Time `json:"checked_at"`
}

func cachePath() string {
	return filepath.Join(config.ConfigDir(), cacheFile)
}

func readCache() *cachedCheck {
	data, err := os.ReadFile(cachePath())
	if err != nil {
		return nil
	}
	var c cachedCheck
	if err := json.Unmarshal(data, &c); err != nil {
		return nil
	}
	if time.Since(c.CheckedAt) > cacheTTL {
		return nil
	}
	return &c
}

func writeCache(version string) {
	c := cachedCheck{
		LatestVersion: version,
		CheckedAt:     time.Now(),
	}
	data, _ := json.Marshal(c)
	os.MkdirAll(filepath.Dir(cachePath()), 0755)
	os.WriteFile(cachePath(), data, 0644)
}

// CheckLatest returns the latest release version from GitHub.
// Uses a file cache to avoid hitting the API on every invocation.
func CheckLatest() (string, error) {
	if c := readCache(); c != nil {
		return c.LatestVersion, nil
	}

	client := &http.Client{Timeout: apiTimeout}
	resp, err := client.Get(fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", repo))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("github api: %s", resp.Status)
	}

	var release Release
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", err
	}

	writeCache(release.TagName)
	return release.TagName, nil
}

// IsNewer returns true if latest is a newer version than current.
func IsNewer(current, latest string) bool {
	cur := parseVersion(current)
	lat := parseVersion(latest)
	if cur == nil || lat == nil {
		return false
	}
	for i := 0; i < 3; i++ {
		if lat[i] > cur[i] {
			return true
		}
		if lat[i] < cur[i] {
			return false
		}
	}
	return false
}

func parseVersion(v string) []int {
	v = strings.TrimPrefix(v, "v")
	parts := strings.SplitN(v, ".", 3)
	if len(parts) != 3 {
		return nil
	}
	nums := make([]int, 3)
	for i, p := range parts {
		n := 0
		for _, c := range p {
			if c < '0' || c > '9' {
				return nil
			}
			n = n*10 + int(c-'0')
		}
		nums[i] = n
	}
	return nums
}

// CheckUpdateNoticeAsync starts a background update check and sends the result
// on the returned channel.
func CheckUpdateNoticeAsync(currentVersion string) <-chan string {
	ch := make(chan string, 1)
	if c := readCache(); c != nil {
		if IsNewer(currentVersion, c.LatestVersion) {
			ch <- c.LatestVersion
		} else {
			ch <- ""
		}
		return ch
	}
	go func() {
		latest, err := CheckLatest()
		if err != nil {
			ch <- ""
			return
		}
		if IsNewer(currentVersion, latest) {
			ch <- latest
		} else {
			ch <- ""
		}
	}()
	return ch
}

// Upgrade downloads and installs the latest (or specified) version.
func Upgrade(targetVersion string) error {
	if targetVersion == "" {
		latest, err := fetchLatest()
		if err != nil {
			return fmt.Errorf("fetching latest version: %w", err)
		}
		targetVersion = latest
	}

	osName := runtime.GOOS
	arch := runtime.GOARCH

	release, err := fetchReleaseAssets(targetVersion)
	if err != nil {
		return err
	}

	assetURL, err := findAssetURL(release, targetVersion, osName, arch)
	if err != nil {
		return err
	}

	checksumURL, err := findChecksumURL(release, targetVersion)
	if err != nil {
		return err
	}

	expectedHash, err := fetchExpectedChecksum(checksumURL, osName, arch)
	if err != nil {
		return fmt.Errorf("fetching checksums: %w", err)
	}

	client := &http.Client{Timeout: fetchTimeout}
	resp, err := client.Get(assetURL)
	if err != nil {
		return fmt.Errorf("downloading: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("download failed: %s", resp.Status)
	}

	tmpDir, err := os.MkdirTemp("", "clasp-upgrade-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	// Determine archive extension
	isZip := strings.HasSuffix(assetURL, ".zip")
	ext := ".tar.gz"
	if isZip {
		ext = ".zip"
	}

	archivePath := filepath.Join(tmpDir, "archive"+ext)
	archiveFile, err := os.Create(archivePath)
	if err != nil {
		return err
	}
	hasher := sha256.New()
	if _, err := io.Copy(archiveFile, io.TeeReader(io.LimitReader(resp.Body, maxBinarySize), hasher)); err != nil {
		archiveFile.Close()
		return fmt.Errorf("downloading archive: %w", err)
	}
	archiveFile.Close()

	actualHash := hex.EncodeToString(hasher.Sum(nil))
	if actualHash != expectedHash {
		return fmt.Errorf("checksum mismatch: expected %s, got %s", expectedHash, actualHash)
	}

	binaryName := "clasp"
	if osName == "windows" {
		binaryName = "clasp.exe"
	}

	tmpBin := filepath.Join(tmpDir, binaryName)
	if isZip {
		if err := extractBinaryFromZip(archivePath, tmpBin, binaryName); err != nil {
			return fmt.Errorf("extracting: %w", err)
		}
	} else {
		archiveReader, err := os.Open(archivePath)
		if err != nil {
			return err
		}
		defer archiveReader.Close()
		if err := extractBinaryFromTarGz(archiveReader, tmpBin, binaryName); err != nil {
			return fmt.Errorf("extracting: %w", err)
		}
	}

	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("finding current binary: %w", err)
	}
	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		return fmt.Errorf("resolving binary path: %w", err)
	}

	if err := replaceBinary(tmpBin, exe); err != nil {
		return err
	}

	os.Remove(cachePath())
	return nil
}

func fetchLatest() (string, error) {
	client := &http.Client{Timeout: apiTimeout}
	resp, err := client.Get(fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", repo))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("github api: %s", resp.Status)
	}

	var release Release
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", err
	}
	return release.TagName, nil
}

type releaseAssets struct {
	Assets []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
	} `json:"assets"`
}

func fetchReleaseAssets(tag string) (*releaseAssets, error) {
	client := &http.Client{Timeout: apiTimeout}
	resp, err := client.Get(fmt.Sprintf("https://api.github.com/repos/%s/releases/tags/%s", repo, tag))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("release %s not found: %s", tag, resp.Status)
	}

	var release releaseAssets
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, err
	}
	return &release, nil
}

func validateAssetURL(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid asset URL: %w", err)
	}
	if u.Scheme != "https" {
		return fmt.Errorf("asset URL must use https, got %s", u.Scheme)
	}
	for _, allowed := range allowedDownloadHosts {
		if u.Host == allowed || strings.HasSuffix(u.Host, "."+allowed) {
			return nil
		}
	}
	return fmt.Errorf("asset URL host %q is not an allowed GitHub domain", u.Host)
}

func findAssetURL(release *releaseAssets, tag, osName, arch string) (string, error) {
	tarSuffix := fmt.Sprintf("_%s_%s.tar.gz", osName, arch)
	zipSuffix := fmt.Sprintf("_%s_%s.zip", osName, arch)
	for _, a := range release.Assets {
		if strings.HasSuffix(a.Name, tarSuffix) || strings.HasSuffix(a.Name, zipSuffix) {
			if err := validateAssetURL(a.BrowserDownloadURL); err != nil {
				return "", err
			}
			return a.BrowserDownloadURL, nil
		}
	}
	return "", fmt.Errorf("no asset found for %s/%s in release %s", osName, arch, tag)
}

func findChecksumURL(release *releaseAssets, tag string) (string, error) {
	for _, a := range release.Assets {
		if a.Name == "checksums.txt" {
			if err := validateAssetURL(a.BrowserDownloadURL); err != nil {
				return "", err
			}
			return a.BrowserDownloadURL, nil
		}
	}
	return "", fmt.Errorf("checksums.txt not found in release %s", tag)
}

func fetchExpectedChecksum(checksumURL, osName, arch string) (string, error) {
	client := &http.Client{Timeout: apiTimeout}
	resp, err := client.Get(checksumURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("downloading checksums: %s", resp.Status)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", err
	}

	tarSuffix := fmt.Sprintf("_%s_%s.tar.gz", osName, arch)
	zipSuffix := fmt.Sprintf("_%s_%s.zip", osName, arch)
	for _, line := range strings.Split(string(body), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) != 2 {
			continue
		}
		if strings.HasSuffix(parts[1], tarSuffix) || strings.HasSuffix(parts[1], zipSuffix) {
			return parts[0], nil
		}
	}
	return "", fmt.Errorf("no checksum found for %s/%s", osName, arch)
}

func extractBinaryFromTarGz(r io.Reader, dest, binaryName string) error {
	gz, err := gzip.NewReader(r)
	if err != nil {
		return err
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if filepath.Base(hdr.Name) == binaryName && hdr.Typeflag == tar.TypeReg {
			if hdr.Size > maxBinarySize {
				return fmt.Errorf("binary too large: %d bytes", hdr.Size)
			}
			f, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
			if err != nil {
				return err
			}
			defer f.Close()
			if _, err := io.Copy(f, io.LimitReader(tr, maxBinarySize)); err != nil {
				return err
			}
			return nil
		}
	}
	return fmt.Errorf("%s binary not found in archive", binaryName)
}

func extractBinaryFromZip(archivePath, dest, binaryName string) error {
	r, err := zip.OpenReader(archivePath)
	if err != nil {
		return err
	}
	defer r.Close()

	for _, f := range r.File {
		if filepath.Base(f.Name) == binaryName {
			if f.UncompressedSize64 > uint64(maxBinarySize) {
				return fmt.Errorf("binary too large: %d bytes", f.UncompressedSize64)
			}
			rc, err := f.Open()
			if err != nil {
				return err
			}
			defer rc.Close()

			out, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
			if err != nil {
				return err
			}
			defer out.Close()
			if _, err := io.Copy(out, io.LimitReader(rc, maxBinarySize)); err != nil {
				return err
			}
			return nil
		}
	}
	return fmt.Errorf("%s binary not found in archive", binaryName)
}

func replaceBinary(src, dst string) error {
	dir := filepath.Dir(dst)
	tmp, err := os.CreateTemp(dir, ".clasp-new-*")
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	tmpPath := tmp.Name()

	srcFile, err := os.Open(src)
	if err != nil {
		os.Remove(tmpPath)
		return err
	}
	defer srcFile.Close()

	if _, err := io.Copy(tmp, srcFile); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return err
	}
	tmp.Close()

	if err := os.Chmod(tmpPath, 0755); err != nil {
		os.Remove(tmpPath)
		return err
	}

	if err := os.Rename(tmpPath, dst); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("replacing binary: %w", err)
	}

	return nil
}
