// Copyright 2019 Google Inc. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"context"
	"encoding/csv"
	"fmt"
	"os"
	"text/template"
	"time"

	"github.com/nammn/go-licenses/v2/internal/third_party/pkgsite/source"
	"github.com/nammn/go-licenses/v2/licenses"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"
	"k8s.io/klog/v2"
)

const (
	UNKNOWN = "Unknown"
)

var (
	reportHelp = "Prints report of all licenses that apply to one or more Go packages and their dependencies."
	reportCmd  = &cobra.Command{
		Use:   "report <package> [package...]",
		Short: reportHelp,
		Long:  reportHelp + packageHelp,
		Args:  cobra.MinimumNArgs(1),
		RunE:  reportMain,
	}

	templateFile string
	skipURL      bool
)

func init() {
	reportCmd.Flags().StringVar(&templateFile, "template", "", "Custom Go template file to use for report")
	reportCmd.Flags().BoolVar(&skipURL, "skip_url", false, "Skip license URL discovery. The LicenseURL field is left as Unknown.")

	rootCmd.AddCommand(reportCmd)
}

type libraryData struct {
	Name         string
	Version      string
	LicensePath  string
	LicenseURL   string
	LicenseNames []string
}

type libraryDataFlat struct {
	Name        string
	Version     string
	LicensePath string
	LicenseURL  string
	LicenseName string
}

// licenseURLResolver resolves the URL of a license file.
type licenseURLResolver func(context.Context, string) (string, error)

// resolveLicenseURL returns the license URL unless skip is set or the library
// has no license file. It is the single decision point for whether the report
// contacts a module host.
func resolveLicenseURL(ctx context.Context, licenseFile string, skip bool, resolve licenseURLResolver) (string, error) {
	if skip || licenseFile == "" {
		return UNKNOWN, nil
	}
	url, err := resolve(ctx, licenseFile)
	if err != nil {
		return UNKNOWN, err
	}
	return url, nil
}

// LicenseText reads and returns the contents of LicensePath, if set
// or an empty string if not.
func (lib libraryDataFlat) LicenseText() (string, error) {
	if lib.LicensePath == "" {
		return "", nil
	}
	data, err := os.ReadFile(lib.LicensePath)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func reportMain(_ *cobra.Command, args []string) error {
	classifier, err := licenses.NewClassifier()
	if err != nil {
		return err
	}

	libs, err := licenses.Libraries(context.Background(), classifier, includeTests, ignore, args...)
	if err != nil {
		return err
	}

	reportData := make([]libraryData, len(libs))
	client := source.NewClient(time.Second * 20)
	group, gctx := errgroup.WithContext(context.Background())
	for idx, lib := range libs {
		idx := idx
		lib := lib

		reportData[idx] = libraryData{
			Name:         lib.Name(),
			Version:      UNKNOWN,
			LicensePath:  UNKNOWN,
			LicenseURL:   UNKNOWN,
			LicenseNames: nil,
		}

		if version := lib.Version(); version != "" {
			reportData[idx].Version = version
		}

		if lib.LicenseFile != "" {
			reportData[idx].LicensePath = lib.LicenseFile
		}

		for _, license := range lib.Licenses {
			reportData[idx].LicenseNames = append(reportData[idx].LicenseNames, license.Name)
		}

		if lib.LicenseFile != "" {
			group.Go(func() error {
				url, err := resolveLicenseURL(gctx, lib.LicenseFile, skipURL, func(ctx context.Context, licenseFile string) (string, error) {
					return lib.FileURL(ctx, client, licenseFile)
				})
				if err == nil {
					reportData[idx].LicenseURL = url
				} else {
					klog.Warningf("Error discovering license URL: %s", err)
				}
				return nil
			})
		}
	}

	if err := group.Wait(); err != nil {
		return err
	}

	// Flatten the report data
	reportDataFlat := make([]libraryDataFlat, 0, len(reportData))
	for _, lib := range reportData {
		if len(lib.LicenseNames) == 0 {
			if lib.LicensePath != UNKNOWN {
				klog.Errorf("Error identifying license in %q: %v", lib.LicensePath, fmt.Errorf("no license found"))
			} else if lib.Version != UNKNOWN {
				klog.Errorf("Error identifying license for version %q of %q: %v", lib.Version, lib.Name, fmt.Errorf("no license found"))
			} else {
				klog.Errorf("Error identifying license for %q: %v", lib.Name, fmt.Errorf("no license found"))
			}
			reportDataFlat = append(reportDataFlat, libraryDataFlat{
				Name:        lib.Name,
				Version:     lib.Version,
				LicensePath: lib.LicensePath,
				LicenseURL:  lib.LicenseURL,
				LicenseName: UNKNOWN,
			})
		} else {
			for _, licenseName := range lib.LicenseNames {
				reportDataFlat = append(reportDataFlat, libraryDataFlat{
					Name:        lib.Name,
					Version:     lib.Version,
					LicensePath: lib.LicensePath,
					LicenseURL:  lib.LicenseURL,
					LicenseName: licenseName,
				})
			}
		}
	}

	if templateFile == "" {
		return reportCSV(reportDataFlat)
	} else {
		return reportTemplate(reportDataFlat)
	}
}

func reportCSV(libs []libraryDataFlat) error {
	writer := csv.NewWriter(os.Stdout)
	for _, lib := range libs {
		if err := writer.Write([]string{lib.Name, lib.LicenseURL, lib.LicenseName}); err != nil {
			return err
		}
	}
	writer.Flush()
	return writer.Error()
}

func reportTemplate(libs []libraryDataFlat) error {
	templateBytes, err := os.ReadFile(templateFile)
	if err != nil {
		return err
	}
	tmpl, err := template.New("").Parse(string(templateBytes))
	if err != nil {
		return err
	}
	return tmpl.Execute(os.Stdout, libs)
}
