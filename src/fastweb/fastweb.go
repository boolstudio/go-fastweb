// vim: set syntax=go autoindent:
// Copyright 2010 Ivan Wong. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
package fastweb

import (
	"fastcgi"
	"os"
	"strings"
	"reflect"
	"unsafe"
	"strconv"
	"fmt"
	"fastweb/template"
	"io/ioutil"
)

type ControllerInterface interface {
	PageTitle() string
	Layout() string
	SetEnv(env *env)
	DefaultAction() string
	Render() os.Error
}

type Controller struct {
	Path        string
	Name        string
	LName       string
	Action      string
	LAction     string
	Params      []string
	ContentType string
	D           interface{}
	Request     *fastcgi.Request
	info        *controllerInfo
	app         *Application
	preRenered  bool
}

func (c *Controller) PageTitle() string { return "" }

func (c *Controller) Layout() string { return "default" }

func (c *Controller) DefaultAction() string { return "index" }

func (c *Controller) SetEnv(env *env) {
	c.Name = env.controller
	c.LName = env.lcontroller
	c.Action = env.action
	c.LAction = env.laction
	c.Path = env.path
	c.Params = env.params
	c.Request = env.request
	c.info = env.cinfo
	c.app = env.app
}

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
		return nil, os.NewError("'" + fname + "' is not a regular file")
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

func (c *Controller) RenderContent() string {
	c.preRender()

	fname := "views/" + c.LName + "/" + c.LAction + ".tpl"
	t, e := loadTemplate(fname)
	if e != nil {
	}

	if t != nil {
		t.Execute(c, c.Request.Stdout)
	}

	return ""
}

func (c *Controller) RenderElement(name string) string {
	fmt.Fprintf(c.Request.Stdout, name)
	return ""
}

func (c *Controller) RenderControllerElement(name string) string {
	fmt.Fprintf(c.Request.Stdout, name)
	return ""
}

func (c *Controller) preRender() {
	if !c.preRenered {
		ct := c.ContentType
		if ct == "" {
			ct = "text/html"
		}
		fmt.Fprintf(c.Request.Stdout, "Content-type: %s\r\n\r\n", ct)
		c.preRenered = true
	}
}

func (c *Controller) Render() os.Error {
	c.preRender()

	fname := "views/layouts/" + c.Layout() + ".tpl"
	t, e := loadTemplate(fname)
	if e != nil {
		c.RenderContent()
	} else {
		t.Execute(c, c.Request.Stdout)
	}

	return nil
}

const (
	IntParam = 1
	StrParam = 2
)

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
	cinfo       *controllerInfo
	app         *Application
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

func getEnv(r *fastcgi.Request) *env {
	var params []string
	var lname string
	var laction string

	path, _ := r.Params["REQUEST_URI"]
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

	return &env{
		path: path,
		controller: name,
		lcontroller: lname,
		action: action,
		laction: laction,
		params: params,
		request: r,
	}
}

func (a *Application) route(r *fastcgi.Request) os.Error {
	env := getEnv(r)

	if env.controller == "" {
		env.controller = a.defaultController
	}

	cinfo, _ := a.controllerMap[env.controller]
	if cinfo == nil {
		return os.NewError("controller class '" + env.controller + "' not found")
	}

	cval := reflect.NewValue(cinfo.controller)
	cval.(*reflect.PtrValue).PointTo(reflect.MakeZero(cinfo.controllerType))
	c := unsafe.Unreflect(cinfo.controllerPtrType, unsafe.Pointer(cval.Addr())).(ControllerInterface)

	if env.action == "" {
		env.action = c.DefaultAction()
	}

	minfo, _ := cinfo.methodMap[env.action]
	if minfo == nil {
		return os.NewError("action '" + env.action + "' is not implemented in controller '" + env.controller + "'")
	}

	if minfo.nparams > len(env.params) {
		return os.NewError("not enough parameter")
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
				return os.NewError(fmt.Sprintf("parameter %d must be an integer, input: %s", i + 1, p))
			}
			pv[i+1] = reflect.NewValue(x)
		}
	}

	env.cinfo = cinfo
	env.app = a

	c.SetEnv(env)

	eval := minfo.method.Call(pv)[0].(*reflect.InterfaceValue)
	if !eval.IsNil() {
		elemval := eval.Elem()
		return unsafe.Unreflect(elemval.Type(), unsafe.Pointer(elemval.Addr())).(os.Error)
	}

	c.Render()

	return nil
}

func (a *Application) Handle(r *fastcgi.Request) bool {
	e := a.route(r)

	if e != nil {
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
		case "PageTitle", "Layout", "SetContext", "Render", "DefaultAction":
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
