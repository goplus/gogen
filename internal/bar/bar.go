/*
 Copyright 2021 The GoPlus Authors (goplus.org)
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

package bar

const (
	GopPackage = true // to indicate this is a Go+ package
)

// -----------------------------------------------------------------------------

type Gamer interface {
	RunLoop()
}

type Game struct {
}

func (p *Game) RunLoop() {
}

func Gopt_Game_Run(game Gamer, resource string) {
	game.RunLoop()
}

func Gops_Game_New() *Game {
	return nil
}

type Info struct {
	id int
}

// -----------------------------------------------------------------------------
