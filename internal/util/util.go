/*
 Copyright 2024 Qiniu Cloud (qiniu.com).

 Licensed under the Apache License, Version 2.0 (the "License");
 you may not use this file except in compliance with the License.
 You may obtain a copy of the License at

     http://www.apache.org/licenses/LICENSE-2.0

 Unless required by applicable law or agreed to in writing, software
 distributed under the License is distributed on an "AS IS" BASIS,
 WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 See the License for the specific language governing permissions and
 limitations under the License.
*/

package util

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/qiniu/x/log"
	"github.com/qiniu/x/xlog"
)

// LimitJoin joins the strings in str with a newline separator until the length of the result is greater than length.
func LimitJoin(str []string, length int) string {
	var result string
	for _, s := range str {
		if strings.TrimSpace(s) == "" {
			continue
		}

		if len(result)+len(s) > length {
			break
		}

		result += s + "\n"
	}

	return result
}

func FileExists(path string) (absPath string, exist bool) {
	fileAbs, err := filepath.Abs(path)
	if err != nil {
		log.Warnf("failed to get absolute path of %s: %v", path, err)
		return "", false
	}

	_, err = os.Stat(fileAbs)
	if err != nil {
		return "", false
	}

	return fileAbs, true
}

type contextKey string

// EventGUIDKey is the key for the event GUID in the context.
const EventGUIDKey contextKey = "event_guid"

// util.FromContext returns a logger from the context.
func FromContext(ctx context.Context) *xlog.Logger {
	eventGUID, ok := ctx.Value(EventGUIDKey).(string)
	if !ok {
		return xlog.New("default")
	}
	return xlog.New(eventGUID)
}

// GetEventGUID returns the event GUID from the context.
func GetEventGUID(ctx context.Context) string {
	eventGUID, ok := ctx.Value(EventGUIDKey).(string)
	if !ok {
		return "default"
	}
	return eventGUID
}

func FindFileWithExt(dir string, ext []string) ([]string, error) {
	var files []string
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		for _, ext := range ext {
			if strings.HasSuffix(info.Name(), ext) {
				absPath, err := filepath.Abs(path)
				if err != nil {
					return err
				}
				files = append(files, absPath)
				break
			}
		}
		return nil
	})
	return files, err
}
