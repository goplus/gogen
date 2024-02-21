/*
 Copyright 2024 The GoPlus Authors (goplus.org)
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

package token

import (
	"go/token"
)

// Token is the set of lexical tokens of the Go/Go+ programming language.
type Token = token.Token

const (
	additional_beg = token.TILDE - 1
	additional_end = token.TILDE + 1

	SRARROW   = additional_beg // -> (single right arrow)
	BIDIARROW = additional_end // <> (bidirectional arrow)
)

func String(tok Token) string {
	if tok >= additional_beg {
		switch tok {
		case SRARROW:
			return "->"
		case BIDIARROW:
			return "<>"
		}
	}
	return tok.String()
}
