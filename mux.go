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

package hugot

import (
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"sync"

	"golang.org/x/net/context"
)

func init() {
	DefaultMux = NewMux("defaultMux", "")
	DefaultMux.httpm = http.DefaultServeMux
}

// Mux is a Handler that multiplexes messages to a set of Command, Hears, and
// Raw handlers.
type Mux struct {
	name string
	desc string

	*sync.RWMutex
	hndlrs   []Handler                         // All the handlers
	rhndlrs  []RawHandler                      // Raw handlers
	bghndlrs []BackgroundHandler               // Long running background handlers
	hears    map[*regexp.Regexp][]HearsHandler // Hearing handlers
	cmds     *CommandMux                       // Command handlers
	httpm    *http.ServeMux                    // http Mux
}

// DefaultMux is a default Mux instance, http Handlers will be added to
// http.DefaultServeMux
var DefaultMux *Mux

// NewMux creates a new Mux.
func NewMux(name, desc string) *Mux {
	mx := &Mux{
		name:     name,
		desc:     desc,
		RWMutex:  &sync.RWMutex{},
		rhndlrs:  []RawHandler{},
		bghndlrs: []BackgroundHandler{},
		hears:    map[*regexp.Regexp][]HearsHandler{},
		cmds:     NewCommandMux(nil),
		httpm:    http.NewServeMux(),
	}
	mx.AddCommandHandler(&muxHelp{mx})
	return mx
}

// StartBackground starts any registered background handlers.
func (mx *Mux) StartBackground(ctx context.Context, w ResponseWriter) {
	mx.RLock()
	defer mx.RUnlock()

	for _, h := range mx.bghndlrs {
		go RunBackgroundHandler(ctx, h, w)
	}
}

// Handle implements the Handler interface. Message will first be passed to
// any registered RawHandlers. If the message has been deemed, by the Adapter
// to have been sent directly to the bot, any comand handlers will be processed.
// Then, if appropriate, the message will be matched against any Hears patterns
// and all matching Heard functions will then be called.
// Any unrecognized errors from the Command handlers will be passed back to the
// user that sent us the message.
func (mx *Mux) Handle(ctx context.Context, w ResponseWriter, m *Message) error {
	mx.RLock()
	defer mx.RUnlock()
	var err error

	// We run all raw message handlers
	for _, rh := range mx.rhndlrs {
		mc := *m
		go rh.Handle(ctx, w, &mc)
	}

	if m.ToBot {
		err = RunCommandHandler(ctx, mx.cmds, w, m)
	}

	if err == ErrSkipHears {
		return nil
	}

	for _, hhs := range mx.hears {
		for _, hh := range hhs {
			mc := *m
			if RunHearsHandler(ctx, hh, w, &mc) {
				err = nil
			}
		}
	}

	if err != nil {
		fmt.Fprintf(w, "error, %s", err.Error())
	}

	return nil
}

// Add adds the provided handler to the DefaultMux
func Add(h Handler) error {
	return DefaultMux.Add(h)
}

// Add a generic handler that supports one or more of the handler
// types. WARNING: This may be removed in the future. Prefer to
// the specific Add*Handler methods.
func (mx *Mux) Add(h Handler) error {
	var used bool
	if h, ok := h.(RawHandler); ok {
		mx.AddRawHandler(h)
		used = true
	}

	if h, ok := h.(BackgroundHandler); ok {
		mx.AddBackgroundHandler(h)
		used = true
	}

	if h, ok := h.(CommandHandler); ok {
		mx.AddCommandHandler(h)
		used = true
	}

	if h, ok := h.(HearsHandler); ok {
		mx.AddHearsHandler(h)
		used = true
	}

	if h, ok := h.(HTTPHandler); ok {
		mx.AddHTTPHandler(h)
		used = true
	}

	mx.Lock()
	defer mx.Unlock()

	if !used {
		return fmt.Errorf("Don't know how to use %T as a handler", h)
	}

	mx.hndlrs = append(mx.hndlrs, h)

	return nil
}

// AddRawHandler adds the provided handler to the DefaultMux
func AddRawHandler(h RawHandler) error {
	return DefaultMux.AddRawHandler(h)
}

// AddRawHandler adds the provided handler to the Mux. All
// messages sent to the mux will be forwarded to this handler.
func (mx *Mux) AddRawHandler(h RawHandler) error {
	mx.Lock()
	defer mx.Unlock()

	if h, ok := h.(RawHandler); ok {
		mx.rhndlrs = append(mx.rhndlrs, h)
	}

	return nil
}

// AddBackgroundHandler adds the provided handler to the DefaultMux
func AddBackgroundHandler(h BackgroundHandler) error {
	return DefaultMux.AddBackgroundHandler(h)
}

// AddBackgroundHandler adds the provided handler to the Mux. It
// will be started with the Mux is started.
func (mx *Mux) AddBackgroundHandler(h BackgroundHandler) error {
	mx.Lock()
	defer mx.Unlock()
	//name, _ := h.Describe()

	mx.bghndlrs = append(mx.bghndlrs, h)

	return nil
}

// AddHearsHandler adds the provided handler to the DefaultMux
func AddHearsHandler(h HearsHandler) error {
	return DefaultMux.AddHearsHandler(h)
}

// AddHearsHandler adds the provided handler to the mux. All
// messages matching the Hears patterns will be forwarded to
// the handler.
func (mx *Mux) AddHearsHandler(h HearsHandler) error {
	mx.Lock()
	defer mx.Unlock()

	r := h.Hears()
	mx.hears[r] = append(mx.hears[r], h)

	return nil
}

// AddCommandHandler adds the provided handler to the DefaultMux
func AddCommandHandler(h CommandHandler) *CommandMux {
	return DefaultMux.AddCommandHandler(h)
}

// AddCommandHandler Adds the provided handler to the mux. The
// returns CommandMux can be used to add sub-commands to this
// command handler.
func (mx *Mux) AddCommandHandler(h CommandHandler) *CommandMux {
	mx.Lock()
	defer mx.Unlock()

	return mx.cmds.AddCommandHandler(h)
}

// Describe implements the Describe method of Handler for
// the Mux
func (mx *Mux) Describe() (string, string) {
	return mx.name, mx.desc
}

// CommandMux is a handler that support "nested" command line
// commands.
type CommandMux struct {
	CommandHandler
	subCmds map[string]*CommandMux
}

// NewCommandMux creates a new CommandMux. The provided base
// command handler will be called first. This can process any
// initial flags if desired. If the base command handler returns
// ErrNextCommand, any command handlers that have been added to
// this mux will then be called with the message Args having
// been appropriately adjusted
func NewCommandMux(base CommandHandler) *CommandMux {
	return &CommandMux{base, map[string]*CommandMux{}}
}

// AddCommandHandler adds a sub-command to an existing CommandMux
func (cx *CommandMux) AddCommandHandler(c CommandHandler) *CommandMux {
	n, _ := c.Describe()

	var subMux *CommandMux
	if ccx, ok := c.(*CommandMux); ok {
		cx.subCmds[n] = ccx
	} else {
		subMux = NewCommandMux(c)
		cx.subCmds[n] = subMux
	}

	return subMux
}

// Command implements the Command handler for a CommandMux
func (cx *CommandMux) Command(ctx context.Context, w ResponseWriter, m *Message) error {
	var err error
	if cx.CommandHandler != nil {
		err = RunCommandHandler(ctx, cx.CommandHandler, w, m)
	} else {
		err = ErrNextCommand
	}

	if err != ErrNextCommand {
		return err
	}

	if len(m.args) == 0 {
		return fmt.Errorf("missing sub-command")
	}

	subs := cx.subCmds

	if cmd, ok := subs[m.args[0]]; ok {
		err = RunCommandHandler(ctx, cmd, w, m)
	} else {
		return ErrUnknownCommand
	}

	return err
}

// SubCommands returns any known subcommands of this command mux
func (cx *CommandMux) SubCommands() map[string]*CommandMux {
	return cx.subCmds
}

// AddHTTPHandler adds the provided handler to the DefaultMux
func AddHTTPHandler(h HTTPHandler) *url.URL {
	return DefaultMux.AddHTTPHandler(h)
}

// AddHTTPHandler registers h as a HTTP handler. The name
// of the Mux, and the name of the handler are used to
// construct a unique URL that can be used to send web
// requests to this handler
func (mx *Mux) AddHTTPHandler(h HTTPHandler) *url.URL {
	mx.Lock()
	defer mx.Unlock()

	n, _ := h.Describe()
	p := fmt.Sprintf("/%s/%s", mx.name, n)
	mx.httpm.Handle(p, h)
	return &url.URL{Path: p}
}

// ServeHTTP iplements http.ServeHTTP for a Mux to allow it to
// act as a web server.
func (mx *Mux) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	mx.httpm.ServeHTTP(w, r)
}
