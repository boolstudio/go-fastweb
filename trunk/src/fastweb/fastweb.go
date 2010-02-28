// vim: set syntax=go autoindent:
// Copyright 2010 Ivan Wong. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
package fastweb

import (
	"container/vector"
	"fastcgi"
	"fastweb/template"
	"fmt"
	"http"
	"io"
	"io/ioutil"
	"log"
	"os"
	"reflect"
	"strconv"
	"strings"
	"unsafe"
)

const (
	IntParam = 1
	StrParam = 2
)

type ControllerInterface interface {
	Init()
	DefaultAction() string
	SetEnv(env *env)
	PreFilter()
	Render()
	SetContext(ctxt ControllerInterface)
}

type Error interface {
	os.Error
	Type() string
}

type ErrorStruct struct {
	typ string
	message string
}

type methodInfo struct {
	name       string
	method     *reflect.FuncValue
	nparams    int
	paramTypes []int
}

type controllerInfo struct {
	name              string
	controller        ControllerInterface
	controllerType    reflect.Type
	controllerPtrType *reflect.PtrType
	methodMap         map[string]*methodInfo
}

type Application struct {
	controllerMap     map[string]*controllerInfo
	defaultController string
}

type env struct {
	path        string
	controller  string
	lcontroller string
	action      string
	laction     string
	params      []string
	request     *fastcgi.Request
	form	    map[string][]string
}

type Controller struct {
	Path        string
	Name        string
	LName       string
	Action      string
	LAction     string
	Params      []string
	PageTitle   string
	Layout      string
	ContentType string
	Form	    map[string][]string
	ctxt        ControllerInterface
	Request     *fastcgi.Request
	preRenered  bool
}

func NewError(typ string, message string) *ErrorStruct {
	return &ErrorStruct{typ, message}
}

func (e *ErrorStruct) String() string { return e.message }

func (e *ErrorStruct) Type() string { return e.typ }

func (c *Controller) Init() {
	c.PageTitle = ""
	c.Layout = "default"
	c.ContentType = "text/html"
}

func (c *Controller) DefaultAction() string { return "Index" }

func (c *Controller) SetEnv(env *env) {
	c.Name = env.controller
	c.LName = env.lcontroller
	c.Action = env.action
	c.LAction = env.laction
	c.Path = env.path
	c.Params = env.params
	c.Request = env.request
	c.Form = env.form
}

func (c *Controller) PreFilter() {}

type tmplInfo struct {
	tmpl  *template.Template
	mtime uint64
}

var tmplCache map[string]*tmplInfo = make(map[string]*tmplInfo)

func loadTemplate(fname string) (*template.Template, os.Error) {
	dir, e := os.Stat(fname)
	if e != nil {
		return nil, e
	}
	if !dir.IsRegular() {
		return nil, NewError("Generic", "'"+fname+"' is not a regular file")
	}
	ti, _ := tmplCache[fname]
	if ti == nil || dir.Mtime_ns > ti.mtime {
		bytes, e := ioutil.ReadFile(fname)
		if e != nil {
			return nil, e
		}
		t, e := template.Parse(string(bytes), nil)
		if e != nil {
			return nil, e
		}
		ti = &tmplInfo{
			mtime: dir.Mtime_ns,
			tmpl: t,
		}
		tmplCache[fname] = ti
	}
	return ti.tmpl, nil
}

func (c *Controller) SetContext(ctxt ControllerInterface) {
	if c.ctxt == nil {
		c.ctxt = ctxt
	}
}

func (c *Controller) preRender() {
	if !c.preRenered {
		io.WriteString(c.Request.Stdout, "Content-type: "+c.ContentType+"\r\n\r\n")
		c.preRenered = true
	}
}

func (c *Controller) RenderContent() string {
	c.preRender()

	fname := "views/" + c.LName + "/" + c.LAction + ".tpl"
	t, e := loadTemplate(fname)
	if e != nil {
		// Dump c.ctxt contents
	}

	if t != nil {
		t.Execute(c.ctxt, c.Request.Stdout)
	}

	return ""
}

func (c* Controller) renderTemplate(fname string) {
	t, e := loadTemplate(fname)
	if e == nil {
		t.Execute(c.ctxt, c.Request.Stdout)
	} else {
		log.Stderrf("failed to load template %s: %s", fname, e)
	}
}

func (c *Controller) RenderElement(name string) string {
	c.renderTemplate("views/elements/" + name + ".tpl")
	return ""
}

func (c *Controller) RenderControllerElement(name string) string {
	c.renderTemplate("views/" + c.LName + "/elements/" + name + ".tpl")
	return ""
}

func (c *Controller) Render() {
	c.preRender()

	fname := "views/layouts/" + c.Layout + ".tpl"
	t, e := loadTemplate(fname)
	if e != nil {
		log.Stderrf("failed to load layout template %s: %s", fname, e)
		c.RenderContent()
	} else {
		t.Execute(c.ctxt, c.Request.Stdout)
	}
}

type ErrorHandler struct {
	Controller
	typ string
}

func NewErrorHandler(e Error, r *fastcgi.Request) *ErrorHandler{
	eh := &ErrorHandler{
		typ: e.Type(),
	}
	eh.Request = r
	eh.Init()
	eh.SetContext(eh)
	return eh
}

func (eh *ErrorHandler) RenderContent() string {
	eh.preRender()

	fname := "views/errors/" + eh.typ + ".tpl"
	t, e := loadTemplate(fname)
	if e != nil {
		var msg string
		switch eh.typ {
		case "PageNotFound":
			msg = "Hmm, the page you’re looking for can't be found."
		default:
			msg = "We're sorry, but there was an error processing your request. Please try again later."
		}
		fmt.Fprintf(eh.Request.Stdout, "%s", msg)
	} else {
		t.Execute(eh, eh.Request.Stdout)
	}

	return ""
}

func titleCase(s string) string {
	if len(s) > 0 {
		parts := strings.Split(s, "_", 0)
		for i, p := range parts {
			parts[i] = strings.ToUpper(string(p[0])) + p[1:len(p)]
		}
		return strings.Join(parts, "")
	}
	return s
}

func parseKeyValueString(m map[string]*vector.StringVector, s string) (os.Error) {
	if s == "" {
		return nil
	}

	// copied from pkg/http/request.go
	for _, kv := range strings.Split(s, "&", 0) {
		if kv == "" {
			continue
		}
		kvPair := strings.Split(kv, "=", 2)

		var key, value string
		var e os.Error
		key, e = http.URLUnescape(kvPair[0])
		if e == nil && len(kvPair) > 1 {
			value, e = http.URLUnescape(kvPair[1])
		}
		if e != nil {
			return e
		}

		vec, ok := m[key]
		if !ok {
			vec = new(vector.StringVector)
			m[key] = vec
		}
		vec.Push(value)
	}

	return nil
}

func parseForm(r *fastcgi.Request) (map[string][]string, os.Error) {
	m := make(map[string]*vector.StringVector)

	s := r.Params["QUERY_STRING"]
	if s != "" {
		e := parseKeyValueString(m, s)
		if e != nil {
			return nil, e
		}
	}

	if r.Params["REQUEST_METHOD"] == "POST" {
		var b []byte
		var e os.Error
		if b, e = ioutil.ReadAll(r.Stdin); e != nil {
			return nil, e
		}
		e = parseKeyValueString(m, string(b))
		if e != nil {
			return nil, e
		}
	}

	var form map[string][]string
	form = make(map[string][]string)

	for k, vec := range m {
		form[k] = vec.Data()
	}

        return form, nil
}

func (a *Application) getEnv(r *fastcgi.Request) *env {
	var params []string
	var lname string
	var laction string

	path, _ := r.Params["REQUEST_URI"]
	p := strings.Split(path, "?", 2)
	if len(p) > 1 {
		path = p[0]
		r.Params["QUERY_STRING"] = p[1]
	}

	pparts := strings.Split(path, "/", 0)
	n := len(pparts)
	if n > 1 {
		lname = pparts[1]
		if n > 2 {
			laction = pparts[2]
			if n > 3 {
				if pparts[n-1] == "" {
					n--
				}
				params = pparts[3:n]
			}
		}
	}

	name := titleCase(lname)
	action := titleCase(laction)

	form, _ := parseForm(r)

	return &env{
		path: path,
		controller: name,
		lcontroller: lname,
		action: action,
		laction: laction,
		params: params,
		request: r,
		form: form,
	}
}

func (a *Application) route(r *fastcgi.Request) os.Error {
	env := a.getEnv(r)

	if env.controller == "" {
		env.controller = a.defaultController
	}

	cinfo, _ := a.controllerMap[env.controller]
	if cinfo == nil {
		return NewError("PageNotFound", "controller class '"+env.controller+"' not found")
	}

	cval := reflect.NewValue(cinfo.controller)
	cval.(*reflect.PtrValue).PointTo(reflect.MakeZero(cinfo.controllerType))
	c := unsafe.Unreflect(cinfo.controllerPtrType, unsafe.Pointer(cval.Addr())).(ControllerInterface)

	if env.action == "" {
		env.action = c.DefaultAction()
	}

	minfo, _ := cinfo.methodMap[env.action]
	if minfo == nil {
		return NewError("PageNotFound", "action '"+env.action+"' is not implemented in controller '"+env.controller+"'")
	}

	if minfo.nparams > len(env.params) {
		return NewError("PageNotFound", "not enough parameter")
	}

	pv := make([]reflect.Value, minfo.nparams+1)
	pv[0] = cval

	for i := 0; i < minfo.nparams; i++ {
		p := env.params[i]
		switch minfo.paramTypes[i] {
		case StrParam:
			pv[i+1] = reflect.NewValue(p)
		case IntParam:
			x, e2 := strconv.Atoi(p)
			if e2 != nil {
				return NewError("PageNotFound", fmt.Sprintf("parameter %d must be an integer, input: %s", i+1, p))
			}
			pv[i+1] = reflect.NewValue(x)
		}
	}

	c.Init()
	c.SetEnv(env)

	c.PreFilter()

	eval := minfo.method.Call(pv)[0].(*reflect.InterfaceValue)
	if !eval.IsNil() {
		elemval := eval.Elem()
		return unsafe.Unreflect(elemval.Type(), unsafe.Pointer(elemval.Addr())).(os.Error)
	}

	c.SetContext(c)
	c.Render()

	return nil
}

func (a *Application) Handle(r *fastcgi.Request) bool {
	e := a.route(r)

	if e != nil {
		var ee Error
		if e, ok := e.(Error); ok {
			ee = e
		} else {
			ee = NewError("Generic", e.String())
		}
		log.Stderrf("%s", e.String())
		eh := NewErrorHandler(ee, r)
		eh.Render()
	}

	return true
}

func NewApplication() *Application {
	return &Application{
		controllerMap: make(map[string]*controllerInfo),
		defaultController: "default",
	}
}

func (a *Application) RegisterController(c ControllerInterface) {
	v := reflect.NewValue(c).(*reflect.PtrValue)
	ptrt := v.Type()
	t := v.Elem().Type()
	name := t.Name()

	mmap := make(map[string]*methodInfo)

	n := ptrt.NumMethod()
	for i := 0; i < n; i++ {
		m := ptrt.Method(i)
		name := m.Name
		switch name {
		case "SetEnv", "Render", "DefaultAction", "Init", "PreFilter", "SetContext":
			continue
		}
		mt := m.Type
		if mt.NumOut() != 1 {
			continue
		}
		switch d := mt.Out(0).(type) {
		case *reflect.InterfaceType:
			if d.PkgPath() != "os" || d.Name() != "Error" {
				continue
			}
		default:
			continue
		}
		nin := mt.NumIn() - 1
		ptypes := make([]int, nin)
		for j := 0; j < nin; j++ {
			switch mt.In(j + 1).(type) {
			case *reflect.IntType:
				ptypes[j] = IntParam
			case *reflect.StringType:
				ptypes[j] = StrParam
			default:
				continue
			}
		}
		mmap[name] = &methodInfo{
			name: name,
			method: m.Func,
			nparams: nin,
			paramTypes: ptypes,
		}
	}

	a.controllerMap[name] = &controllerInfo{
		name: name,
		controller: c,
		controllerType: t,
		controllerPtrType: ptrt.(*reflect.PtrType),
		methodMap: mmap,
	}
}

func (a *Application) Run(addr string) os.Error {
	return fastcgi.RunStandalone(addr, a)
}
