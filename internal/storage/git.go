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

package storage

import (
	"context"
)

type GitStorage struct{}

func NewGitStorage() (Storage, error) {
	return &GitStorage{}, nil
}

func (g *GitStorage) Write(ctx context.Context, key string, content []byte) error {
	return nil
}

func (g *GitStorage) Read(ctx context.Context, key string) ([]byte, error) {
	return nil, nil
}
