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

package linters

import (
	"fmt"
	"testing"
)

func TestParsePatch(t *testing.T) {
	patch := "@@ -132,7 +132,7 @@ module Test @@ -1000,7 +1000,7 @@ module Test"
	fmt.Println(ParsePatch(patch))
}

func TestInHunk(t *testing.T) {
	c := FileHunkChecker{
		Hunks: map[string][]Hunk{
			"testfilename": {
				{
					StartLine: 100,
					EndLine:   105,
				},
				{
					StartLine: 200,
					EndLine:   205,
				},
			},
		},
	}
	tsc := []struct {
		conditionMsg string
		filename     string
		startLine    int
		line         int
		expected     bool
	}{
		{
			"startline == 0 &&   line < Hunk.StartLine1 < Hunk.EndLine1 < Hunk.StartLine2  <Hunk.EndLine2",
			"testfilename",
			0,
			99,
			false,
		},

		{
			"startline == 0 &&  Hunk.StartLine1 < line < Hunk.EndLine1 < Hunk.StartLine2  <Hunk.EndLine2",
			"testfilename",
			0,
			101,
			true,
		},
		{
			"startline == 0 &&  Hunk.StartLine < Hunk.EndLine <line < Hunk.StartLine2  <Hunk.EndLine2",
			"testfilename",
			0,
			106,
			false,
		},
		{
			"startline == 0 &&  Hunk.StartLine1  < Hunk.EndLine1 < Hunk.StartLine2 < line <Hunk.EndLine2",
			"testfilename",
			0,
			201,
			true,
		},
		{
			"startline == 0 &&  Hunk.StartLine1  < Hunk.EndLine1 < Hunk.StartLine2 <Hunk.EndLine2 < line  ",
			"testfilename",
			0,
			206,
			false,
		},
		{
			"startline != 0 &&  startLine <  line < Hunk.StartLine1  <Hunk.EndLine1 < Hunk.StartLine2  <Hunk.EndLine2",
			"testfilename",
			89,
			90,
			false,
		},
		{
			"startline != 0 &&  startLine  < Hunk.StartLine1 <  line < Hunk.EndLine1 < Hunk.StartLine2  < Hunk.EndLine2",
			"testfilename",
			89,
			102,
			false,
		},
		{
			"startline != 0 &&  startLine  < Hunk.StartLine1  < Hunk.EndLine1 < line < Hunk.StartLine2  < Hunk.EndLine2",
			"testfilename",
			89,
			110,
			false,
		},

		{
			"startline != 0 &&  startLine  < Hunk.StartLine1  < Hunk.EndLine1  < Hunk.StartLine2 < line < Hunk.EndLine2",
			"testfilename",
			89,
			202,
			false,
		},

		{
			"startline != 0 &&  startLine  < Hunk.StartLine1  < Hunk.EndLine1  < Hunk.StartLine2  < Hunk.EndLine2 < line",
			"testfilename",
			89,
			210,
			false,
		},

		{
			"startline != 0 &&  Hunk.StartLine1  <  startLine  < line < Hunk.EndLine1  < Hunk.StartLine2  < Hunk.EndLine2 ",
			"testfilename",
			101,
			105,
			true,
		},

		{
			"startline != 0 &&  Hunk.StartLine1  <  startLine  < Hunk.EndLine1 < line  < Hunk.StartLine2  < Hunk.EndLine2 ",
			"testfilename",
			101,
			110,
			false,
		},

		{
			"startline != 0 &&  Hunk.StartLine1  <  startLine  < Hunk.EndLine1   < Hunk.StartLine2 < line < Hunk.EndLine2 ",
			"testfilename",
			101,
			201,
			false,
		},

		{
			"startline != 0 &&  Hunk.StartLine1  <  startLine  < Hunk.EndLine1   < Hunk.StartLine2  < Hunk.EndLine2 < line",
			"testfilename",
			101,
			210,
			false,
		},

		{
			"startline != 0 &&  Hunk.StartLine1  < Hunk.EndLine1  <  startLine < line  < Hunk.StartLine2  < Hunk.EndLine2 ",
			"testfilename",
			150,
			151,
			false,
		},

		{
			"startline != 0 &&  Hunk.StartLine1  < Hunk.EndLine1  <  startLine  < Hunk.StartLine2 < line  < Hunk.EndLine2 ",
			"testfilename",
			150,
			201,
			false,
		},

		{
			"startline != 0 &&  Hunk.StartLine1  < Hunk.EndLine1  <  startLine  < Hunk.StartLine2  < Hunk.EndLine2 < line",
			"testfilename",
			150,
			210,
			false,
		},

		{
			"startline != 0 &&  Hunk.StartLine1  < Hunk.EndLine1  < Hunk.StartLine2  <  startLine < line  < Hunk.EndLine2",
			"testfilename",
			201,
			203,
			true,
		},

		{
			"startline != 0 &&  Hunk.StartLine1  < Hunk.EndLine1  < Hunk.StartLine2  <  startLine  < Hunk.EndLine2 < line ",
			"testfilename",
			201,
			210,
			false,
		},

		{
			"startline != 0 &&  Hunk.StartLine1  < Hunk.EndLine1  < Hunk.StartLine2  < Hunk.EndLine2  <  startLine  < line ",
			"testfilename",
			210,
			211,
			false,
		},

		{
			"finename can not find in hunks",
			"testxxxx",
			110,
			111,
			false,
		},
	}

	for _, v := range tsc {
		res := c.InHunk(v.filename, v.line, v.startLine)
		if res != v.expected {
			t.Errorf("The conditions (*%s*) for the evaluation are not satisfied.\n Expected: %v, got: %v", v.conditionMsg, v.expected, res)
		}
	}
}
