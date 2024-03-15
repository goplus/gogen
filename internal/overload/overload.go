/*
 Copyright 2023 The GoPlus Authors (goplus.org)
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

package overload

const (
	GopPackage = true // to indicate this is a Go+ package
)

// -----------------------------------------------------------------------------

func Put2__0(a int)      {}
func PutInt(a int)       {}
func PutString(a string) {}

const Gopo_Put = "PutInt,PutString"
const Gopo__Put2 = ",PutString"

// -----------------------------------------------------------------------------

type Game struct{}

func (p *Game) Run2__0(a int)      {}
func (p *Game) RunInt(a int)       {}
func (p *Game) RunString(a string) {}

func RunGame(*Game, string) {}

const Gopo__Game__Run = ".RunInt,.RunString"
const Gopo_Game_Run2 = ",.RunString"
const Gopo_Game_Run3 = ".RunInt,RunGame"

// -----------------------------------------------------------------------------
