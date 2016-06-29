// Copyright (c) 2016 Tristan Colgate-McFarlane
//
// This file is part of hugot.
//
// hugot is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// hugot is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with hugot.  If not, see <http://www.gnu.org/licenses/>.

package shell

import (
	"context"
	"log"

	"fmt"
	"math/rand"
	"time"

	"github.com/tcolgate/hugot"

	"github.com/chzyer/readline"
)

type shell struct {
}

func New() (hugot.Adapter, error) {
	return nil, nil
}

func (b *shell) Send(ctx context.Context, m *hugot.Message) {
}

func (s *shell) Receive() <-chan *hugot.Message {
	return nil
}

func main() {
	rl, err := readline.NewEx(&readline.Config{
		UniqueEditLine: true,
	})
	if err != nil {
		panic(err)
	}
	defer rl.Close()

	rl.SetPrompt("username: ")
	username, err := rl.Readline()
	if err != nil {
		return
	}
	rl.ResetHistory()
	log.SetOutput(rl.Stderr())

	fmt.Fprintln(rl, "Hi,", username+"! My name is Dave.")
	rl.SetPrompt(username + "> ")

	done := make(chan struct{})
	go func() {
		rand.Seed(time.Now().Unix())
	loop:
		for {
			select {
			case <-time.After(time.Duration(rand.Intn(20)) * 100 * time.Millisecond):
			case <-done:
				break loop
			}
			log.Println("Dave:", "hello")
		}
		log.Println("Dave:", "bye")
		done <- struct{}{}
	}()

	for {
		ln := rl.Line()
		if ln.CanContinue() {
			continue
		} else if ln.CanBreak() {
			break
		}
		log.Println(username+":", ln.Line)
	}
	rl.Clean()
	done <- struct{}{}
	<-done
}
