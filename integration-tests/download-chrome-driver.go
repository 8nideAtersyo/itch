package main

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/pkg/errors"
)

func downloadChromeDriver(r *runner) error {
	driverCache := filepath.Join(r.cwd, ".chromedriver")
	ext := ".exe"
	if runtime.GOOS != "windows" {
		ext = ""
	}

	driverExe := filepath.Join(driverCache, "chromedriver"+ext)
	r.chromeDriverExe = driverExe

	getChromeDriverVersion := func() (string, error) {
		out, err := exec.Command(driverExe, "--version").CombinedOutput()
		if err != nil {
			return "", errors.WithMessage(err, "getting chromedriver version")
		}
		return strings.TrimSpace(string(out)), nil
	}

	currentVersion, err := getChromeDriverVersion()
	if err == nil {
		if currentVersion == chromeDriverVersionString {
			r.logf("Good version found, keeping it: %s", currentVersion)
			return nil
		}
		r.logf("Found (%s) but expected (%s)", currentVersion, chromeDriverVersion)
	} else {
		r.logf("No chrome driver version found")
	}

	err = os.RemoveAll(driverCache)
	if err != nil {
		return errors.WithMessage(err, "removing webdriver cache")
	}

	r.logf("Downloading chromedriver...")
	err = os.MkdirAll(driverCache, 0755)
	if err != nil {
		return errors.WithMessage(err, "making chromedriver cache")
	}

	url := chromeDriverURL(r)
	r.logf("Downloading from %s", url)

	req, err := http.Get(url)
	if err != nil {
		return errors.WithMessage(err, "downloading chrome driver")
	}

	if req.StatusCode != 200 {
		err = fmt.Errorf("Got HTTP %d when trying to download %s", req.StatusCode, url)
		return err
	}

	defer req.Body.Close()

	buf, err := ioutil.ReadAll(req.Body)
	if err != nil {
		return errors.WithMessage(err, "downloading chromedriver")
	}

	r.logf("Extracting chromedriver...")
	zf, err := zip.NewReader(bytes.NewReader(buf), int64(len(buf)))
	if err != nil {
		return errors.WithMessage(err, "opening chromedriver zip")
	}

	for _, f := range zf.File {
		err = func(f *zip.File) error {
			r, err := f.Open()
			if err != nil {
				return errors.WithMessage(err, "opening entry in chromedriver zip")
			}
			defer r.Close()

			name := filepath.Join(driverCache, f.Name)
			mode := f.FileInfo().Mode()
			flags := os.O_WRONLY | os.O_CREATE | os.O_TRUNC
			w, err := os.OpenFile(name, flags, mode)
			if err != nil {
				return errors.WithMessage(err, "creating chromedriver entry file")
			}
			defer w.Close()

			_, err = io.Copy(w, r)
			if err != nil {
				return errors.WithMessage(err, "writing chromedriver entry file")
			}

			return nil
		}(f)

		if err != nil {
			return errors.WithStack(err)
		}
	}

	currentVersion, err = getChromeDriverVersion()
	must(err)
	r.logf("%s", currentVersion)

	return nil
}

// electron 6.0.0-beta.12 ships with Chromium 76.0.3809.54
// we need ChromeDriver 76.0.3809.25
const chromeDriverVersion = "76.0.3809.25"
const chromeDriverVersionString = "ChromeDriver 76.0.3809.25 (a0c95f440512e06df1c9c206f2d79cc20be18bb1-refs/branch-heads/3809@{#271})"

func chromeDriverURL(r *runner) string {
	tag := chromeDriverVersion

	suffix := ""
	switch runtime.GOOS {
	case "windows":
		suffix = "win32"
	case "linux":
		suffix = "linux64"
	case "darwin":
		suffix = "mac64"
	}

	dl := "https://chromedriver.storage.googleapis.com"
	return fmt.Sprintf("%s/%s/chromedriver_%s.zip", dl, tag, suffix)
}
