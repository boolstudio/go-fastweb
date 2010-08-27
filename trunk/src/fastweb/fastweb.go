// vim: set syntax=go autoindent:
// Copyright 2010 Ivan Wong. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
package fastweb

import (
	"bufio"
	"bytes"
	"container/vector"
	"go-fastcgi.googlecode.com/svn/trunk/src/fastcgi"
	"fmt"
	"http"
	"io"
	"io/ioutil"
	"log"
	"os"
	"rand"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"
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
	StartSession()
	CloseSession()
}

type Error interface {
	os.Error
	Type() string
}

type ErrorStruct struct {
	typ     string
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
	form        map[string][]string
	upload      map[string][](*Upload)
	cookies     map[string]string
}

type Upload struct {
	File	 *os.File
	Filename string
}

type cookie struct {
	value string
	expire *time.Time
	path string
	domain string
	secure bool
	httpOnly bool
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
	Form        map[string][]string
	Upload      map[string][]*Upload
	Cookies	    map[string]string
	Session	    *Session
	setCookies  map[string]*cookie
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
	c.Upload = env.upload
	c.Cookies = env.cookies
}

func (c *Controller) PreFilter() {}

type tmplInfo struct {
	tmpl  *Template
	mtime int64
}

var tmplCache map[string]*tmplInfo = make(map[string]*tmplInfo)

func loadTemplate(fname string) (*Template, os.Error) {
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
		t, e := Parse(string(bytes), nil)
		if e != nil {
			return nil, e
		}
		ti = &tmplInfo{
			mtime: dir.Mtime_ns,
			tmpl:  t,
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
		io.WriteString(c.Request.Stdout, "Content-type: "+c.ContentType+"\r\n")

		if c.setCookies != nil {
			for k, ck := range c.setCookies {
				s := "Set-Cookie: " + k + "=" + http.URLEscape(ck.value)
				if ck.expire != nil {
					s += "; expire=" + ck.expire.Format(time.RFC1123)
				}
				if ck.path != "" {
					s += "; path=" + ck.path
				}
				if ck.domain != "" {
					s += "; path=" + ck.domain
				}
				if ck.secure {
					s += "; secure"
				}
				if ck.httpOnly {
					s += "; HttpOnly"
				}
				io.WriteString(c.Request.Stdout, s+"\r\n")
			}
		}

		io.WriteString(c.Request.Stdout, "\r\n")
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

func (c *Controller) renderTemplate(fname string) {
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

	if len(c.Layout) == 0 {
		c.RenderContent()
		return
	}

	fname := "views/layouts/" + c.Layout + ".tpl"
	t, e := loadTemplate(fname)
	if e != nil {
		log.Stderrf("failed to load layout template %s: %s", fname, e)
		c.RenderContent()
	} else {
		t.Execute(c.ctxt, c.Request.Stdout)
	}
}

func (c *Controller) SetCookie(key string, value string) {
	c.SetCookieFull(key, value, nil, "", "", false, false)
}

func (c *Controller) SetCookieFull(key string, value string, expire *time.Time, path string, domain string, secure bool, httpOnly bool) {
	if c.setCookies == nil {
		c.setCookies = make(map[string]*cookie)
	}

	c.setCookies[key] = &cookie{
		value: value,
		expire: expire,
		path: path,
		domain: domain,
		secure: secure,
		httpOnly: httpOnly,
	}
}

func (c *Controller) StartSession() {
	if c.Session != nil {
		return
	}

	c.Session = GetSession(c)
}

func (c *Controller) CloseSession() {
	if c.Session == nil {
		return
	}

	c.Session.Close()
}

type ErrorHandler struct {
	Controller
	typ string
}

func NewErrorHandler(e Error, r *fastcgi.Request) *ErrorHandler {
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
		parts := strings.Split(s, "_", -1)
		for i, p := range parts {
			parts[i] = strings.ToUpper(string(p[0])) + p[1:len(p)]
		}
		return strings.Join(parts, "")
	}
	return s
}

func deTitleCase(s string) string {
	if len(s) > 0 {
		var o string
		for i, c := range s {
			if c >= 'A' && c <= 'Z' {
				if i > 0 {
					o += "_";
				}
				o += strings.ToLower(string(c));
			} else {
				o += string(c)
			}
		}
		return o
	}
	return s
}

func parseKeyValueString(m map[string]*vector.StringVector, s string) os.Error {
	if s == "" {
		return nil
	}

	// copied from pkg/http/request.go
	for _, kv := range strings.Split(s, "&", -1) {
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

var boundaryRE, _ = regexp.Compile("boundary=\"?([^\";,]+)\"?")

type multipartReader struct {
	rd   io.Reader
	bd   []byte
	buf  []byte
	head int
	tail int
	eof  bool
	done bool
}

func newMultipartReader(rd io.Reader, bd string) *multipartReader {
	return &multipartReader{
		rd:   rd,
		bd:   []byte("\r\n--" + bd),
		buf:  make([]byte, 4096),
		head: 0,
		tail: 0,
	}
}

func (md *multipartReader) finished() bool { return md.done }

func (md *multipartReader) read(delim []byte) ([]byte, os.Error) {
	if md.done {
		return nil, os.EOF
	}

	if !md.eof && md.tail < len(md.buf) {
		n, e := md.rd.Read(md.buf[md.tail:len(md.buf)])
		if e != nil {
			if e != os.EOF {
				return nil, e
			}
			md.eof = true
		}
		md.tail += n
	}

	if i := bytes.Index(md.buf[md.head:md.tail], delim); i >= 0 {
		s := make([]byte, i)
		copy(s, md.buf[md.head:md.head+i])
		md.head += i + len(delim)
		return s, os.EOF
	}

	if md.eof {
		md.done = true
		return md.buf[md.head:md.tail], os.EOF
	}

	bf := md.tail - md.head
	keep := len(delim) - 1
	if keep > bf {
		keep = bf
	}
	stop := md.tail - keep
	n := stop - md.head
	s := make([]byte, n)
	if n > 0 {
		copy(s, md.buf[md.head:stop])
	}

	copy(md.buf[0:keep], md.buf[stop:md.tail])
	md.head = 0
	md.tail = keep

	return s, nil
}

func (md *multipartReader) readFirstLine() { md.readStringUntil(md.bd[2:len(md.bd)], false) }

var crlf2 = []byte{'\r', '\n', '\r', '\n'}

type byteConsumer func([]byte) os.Error

func (md *multipartReader) readUntil(delim []byte, checkEnd bool, f byteConsumer) os.Error {
	for {
		b, e := md.read(delim)
		if b != nil {
			if e := f(b); e != nil {
				return e
			}
		}
		if e != nil {
			if e == os.EOF {
				if checkEnd && md.tail-md.head >= 2 && string(md.buf[md.head:md.head+2]) == "--" {
					md.done = true
				}
				break
			}
			return e
		}
	}

	return nil
}

func (md *multipartReader) readStringUntil(delim []byte, checkEnd bool) (string, os.Error) {
	var s string
	e := md.readUntil(delim, checkEnd, func(b []byte) os.Error {
		s += string(b)
		return nil
	})
	return s, e
}

type hdrInfo struct {
	key string
	val string
	attribs map[string]string
}

func parseHeader(line string) *hdrInfo {
	var key, attrib string
	var hdr *hdrInfo
	var attribs map[string]string
	var j int
	phase := 0
	line += ";"
	for i, c := range line {
		switch phase {
		case 0:
			if c == ':' {
				key = strings.TrimSpace(line[0:i])
				phase++
				j = i + 1
			}
		case 1:
			if c == ';' {
				attribs = make(map[string]string)
				hdr = &hdrInfo {
					key: key,
					val: strings.TrimSpace(line[j:i]),
					attribs: attribs,
				}
				phase++
				j = i + 1
			}
		case 2:
			if c == '=' {
				attrib = strings.TrimSpace(line[j:i])
				phase++
				j = i + 1
			}
		case 3:
			if c == '"' {
				phase++
				j = i + 1
			} else if c == ';' {
				attribs[attrib] = strings.TrimSpace(line[j:i])
				phase = 2
				j = i + 1
			}
		case 4:
			if c == '\\' {
				phase++
			} else if c == '"' {
				attribs[attrib] = line[j:i]
				phase += 2
			}
		case 5:
			phase--
		case 6:
			if c == ';' {
				phase = 2
				j = i + 1
			}
		}
	}
	return hdr
}

func (md *multipartReader) readHeaders() (map[string]*hdrInfo, os.Error) {
	s, _ := md.readStringUntil(crlf2, false)
	lines := strings.Split(s[2:len(s)], "\r\n", -1)
	hdrs := make(map[string]*hdrInfo)
	for _, line := range lines {
		if hdr := parseHeader(line); hdr != nil {
			hdrs[hdr.key] = hdr
		}
	}
	return hdrs, nil
}


func (md *multipartReader) readBody() (string, os.Error) {
	return md.readStringUntil(md.bd, true)
}

var pads = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

func tempfile() (*os.File, os.Error) {
	tmpdir := os.Getenv("TMPDIR")
	if tmpdir == "" {
		tmpdir = "/tmp"
	}

	for {
		var s string
		for i := 0; i < 10; i++ {
			s += string(pads[rand.Int() % len(pads)])
		}
		file, e := os.Open(tmpdir + "/fastweb." + s, os.O_WRONLY | os.O_CREATE | os.O_EXCL, 0600)
		if e == nil {
			return file, e
		}
		pe, ok := e.(*os.PathError)
		if !ok || pe.Error != os.EEXIST {
			return nil, e
		}
	}

	return nil, nil
}

func parseMultipartForm(m map[string]*vector.StringVector, u map[string]*vector.Vector, r *fastcgi.Request) os.Error {
	ct := r.Params["CONTENT_TYPE"]
	a := boundaryRE.FindStringSubmatchIndex(ct)
	if len(a) < 4 {
		return os.NewError("can't find boundary in content type")
	}
	b := ct[a[2]:a[3]]
	md := newMultipartReader(r.Stdin, b)
	md.readFirstLine()
	for !md.finished() {
		hdrs, e := md.readHeaders()
		if e != nil {
			return e
		}
		cd, ok := hdrs["Content-Disposition"]
		if !ok {
			return os.NewError("can't find Content-Disposition")
		}
		name, ok := cd.attribs["name"]
		if !ok {
			return os.NewError("can't find attrib 'name' in Content-Disposition")
		}
		filename, ok := cd.attribs["filename"]
		if ok {
			vec, ok := u[name]
			if !ok {
				vec = new(vector.Vector)
				u[name] = vec
			}

			file, e := tempfile()
			if e != nil {
				return e
			}
			wr := bufio.NewWriter(file)
			fname := file.Name()
			md.readUntil(md.bd, true, func(b []byte) os.Error {
				if _, e := wr.Write(b); e != nil {
					return e
				}
				return nil
			})
			wr.Flush()
			// to flush (system) buffer, re-open immediately
			file.Close()
			file, _ = os.Open(fname, os.O_RDONLY, 0)

			vec.Push(&Upload{
				File: file,
				Filename: filename,
			})
		} else {
			vec, ok := m[name]
			if !ok {
				vec = new(vector.StringVector)
				m[name] = vec
			}
			s, e := md.readBody()
			if e != nil {
				return e
			}
			vec.Push(s)
		}
	}
	return nil
}

func parseForm(r *fastcgi.Request) (map[string][]string, map[string][]*Upload, os.Error) {
	m := make(map[string]*vector.StringVector)
	u := make(map[string]*vector.Vector)

	s := r.Params["QUERY_STRING"]
	if s != "" {
		e := parseKeyValueString(m, s)
		if e != nil {
			return nil, nil, e
		}
	}

	if r.Params["REQUEST_METHOD"] == "POST" {
		switch ct := r.Params["CONTENT_TYPE"]; true {
		case strings.HasPrefix(ct, "application/x-www-form-urlencoded") && (len(ct) == 33 || ct[33] == ';'):
			var b []byte
			var e os.Error
			if b, e = ioutil.ReadAll(r.Stdin); e != nil {
				return nil, nil, e
			}
			e = parseKeyValueString(m, string(b))
			if e != nil {
				return nil, nil, e
			}
		case strings.HasPrefix(ct, "multipart/form-data"):
			e := parseMultipartForm(m, u, r)
			if e != nil {
				return nil, nil, e
			}
		default:
			log.Stderrf("unknown content type '%s'", ct)
		}
	}

	form := make(map[string][]string)
	for k, vec := range m {
		form[k] = vec.Copy()
	}

	upload := make(map[string][]*Upload)
	for k, vec := range u {
		d := vec.Copy()
		v := make([]*Upload, len(d))
		for i, u := range d {
			v[i] = u.(*Upload)
		}
		upload[k] = v
	}

	return form, upload, nil
}

func parseCookies(r *fastcgi.Request) (map[string]string, os.Error) {
	cookies := make(map[string]string)

	if s, ok := r.Params["HTTP_COOKIE"]; ok {
		var key string
		phase := 0
		j := 0
		s += ";"
		for i, c := range s {
			switch phase {
			case 0:
				if c == '=' {
					key = strings.TrimSpace(s[j:i])
					j = i + 1
					phase++
				}
			case 1:
				if c == ';' {
					v, e := http.URLUnescape(s[j:i])
					if e != nil {
						return cookies, e
					}
					cookies[key] = v
					phase = 0
					j = i + 1
				}
			}
		}
	}

	return cookies, nil
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

	pparts := strings.Split(path, "/", -1)
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

	form, upload, e := parseForm(r)
	if e != nil {
		log.Stderrf("failed to parse form: %s", e.String())
	}

	cookies, e := parseCookies(r)
	if e != nil {
		log.Stderrf("failed to parse cookies: %s", e.String())
	}

	return &env{
		path:        path,
		controller:  name,
		lcontroller: lname,
		action:      action,
		laction:     laction,
		params:      params,
		request:     r,
		form:        form,
		upload:      upload,
		cookies:     cookies,
	}
}

func (a *Application) route(r *fastcgi.Request) os.Error {
	env := a.getEnv(r)

	if env.controller == "" {
		env.controller = a.defaultController
		env.lcontroller = deTitleCase(env.controller)
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
		env.laction = deTitleCase(env.action)
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

	c.CloseSession()

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
		controllerMap:     make(map[string]*controllerInfo),
		defaultController: "Default",
	}
}

func (a *Application) RegisterController(c ControllerInterface) {
	v := reflect.NewValue(c).(*reflect.PtrValue)
	pt := v.Type()
	t := v.Elem().Type()
	name := t.Name()

	mmap := make(map[string]*methodInfo)

	n := pt.NumMethod()
	for i := 0; i < n; i++ {
		m := pt.Method(i)
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
			name:       name,
			method:     m.Func,
			nparams:    nin,
			paramTypes: ptypes,
		}
	}

	a.controllerMap[name] = &controllerInfo{
		name:              name,
		controller:        c,
		controllerType:    t,
		controllerPtrType: pt.(*reflect.PtrType),
		methodMap:         mmap,
	}
}

func (a *Application) Run(addr string) os.Error {
	rand.Seed(time.Nanoseconds())
	return fastcgi.RunStandalone(addr, a)
}

