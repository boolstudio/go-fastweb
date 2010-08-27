// vim: set syntax=go autoindent:
// Copyright 2010 Ivan Wong. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
package fastweb

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"rand"
	"reflect"
	"strconv"
)

var SessionFilePath = "/tmp"

func escape(s string) string {
	quoteCnt := 0
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '"' {
			quoteCnt++
		}
	}

	if quoteCnt == 0 {
		return s
	}

	t := make([]byte, len(s)+quoteCnt)
	j := 0
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '"' {
			t[j] = '\\'
			j++
			t[j] = '"'
			j++
		} else {
			t[j] = c
			j++
		}
	}
	return string(t)
}

type sliceValue interface {
	Len() int
	Elem(i int) reflect.Value
}

func serializeSlice(wr io.Writer, v sliceValue, tstr string) os.Error {
	_, e := io.WriteString(wr, tstr+":{")
	if e != nil {
		return e
	}
	n := v.Len()
	for i := 0; i < n; i++ {
		e := _serialize(wr, v.Elem(i))
		if e != nil {
			return e
		}
	}
	_, e = io.WriteString(wr, "}")
	return e
}

func writeString(wr io.Writer, s string) os.Error {
	_, e := io.WriteString(wr, "s:\""+escape(s)+"\";")
	return e
}

func _serialize(wr io.Writer, v reflect.Value) os.Error {
	var e os.Error
	switch vv := v.(type) {
	case *reflect.IntValue:
		_, e = io.WriteString(wr, "i:"+strconv.Itoa(int(vv.Get()))+";")
	case *reflect.StringValue:
		writeString(wr, vv.Get())
	case *reflect.SliceValue:
		var tstr string
		et := v.Type().(*reflect.SliceType).Elem()
		switch et.(type) {
		case *reflect.IntType:
			tstr = "i"
		case *reflect.StringType:
			tstr = "s"
		case *reflect.InterfaceType:
			tstr = "*"
		default:
			return os.NewError("only []int, []string, []interface{} are supported")
		}
		e = serializeSlice(wr, vv, "a"+tstr+strconv.Itoa(vv.Len()))
	case *reflect.MapValue:
		var kstr string
		var estr string
		t := v.Type().(*reflect.MapType)
		kt := t.Key()
		switch kt.(type) {
		case *reflect.IntType:
			kstr = "i"
		case *reflect.StringType:
			kstr = "s"
		default:
			return os.NewError("only map[int] or map[string] are supported")
		}
		et := t.Elem()
		switch et.(type) {
		case *reflect.IntType:
			estr = "i"
		case *reflect.StringType:
			estr = "s"
		case *reflect.InterfaceType:
			estr = "*"
		default:
			return os.NewError("only map[]int, map[]string or map[]interface{} are supported")
		}
		_, e := io.WriteString(wr, "m"+kstr+estr+":{")
		if e != nil {
			return e
		}
		keys := vv.Keys()
		for _, k := range keys {
			e := _serialize(wr, k)
			if e != nil {
				return e
			}
			e = _serialize(wr, vv.Elem(k))
			if e != nil {
				return e
			}
		}
		_, e = io.WriteString(wr, "}")
	case *reflect.InterfaceValue:
		e = _serialize(wr, vv.Elem())
	case *reflect.PtrValue:
		sv, ok := vv.Elem().(*reflect.StructValue)
		if !ok {
			return os.NewError("value not support")
		}
		t := sv.Type().(*reflect.StructType)
		n := t.NumField()
		_, e := io.WriteString(wr, "t:{")
		if e != nil {
			return e
		}
		for i := 0; i < n; i++ {
			e := writeString(wr, t.Field(i).Name)
			if e != nil {
				return e
			}
			e = _serialize(wr, sv.Field(i))
			if e != nil {
				return e
			}
		}
		_, e = io.WriteString(wr, "}")
	default:
		return os.NewError("value not support")
	}
	return e
}

func serialize(filename string, perm uint32, data interface{}) os.Error {
	file, e := os.Open(filename, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, perm)
	if e != nil {
		return e
	}

	wr := bufio.NewWriter(file)

	e = _serialize(wr, reflect.NewValue(data))
	wr.Flush()
	file.Close()

	return e
}

func readInt(rd *bufio.Reader) (int, os.Error) {
	var s string
	for {
		c, _, e := rd.ReadRune()
		if e != nil {
			return 0, e
		}
		if c == ':' {
			return strconv.Atoi(s)
		} else {
			s += string(c)
		}
	}
	return 0, nil
}

func readString(rd *bufio.Reader) (string, os.Error) {
	var s string
	phase := 0
	for {
		c, _, e := rd.ReadRune()
		if e != nil {
			return s, e
		}
		switch phase {
		case 0:
			if c == '"' {
				phase++
			} else {
				return "", os.NewError("string without open brace?")
			}
		case 1:
			if c == '\\' {
				phase++
			} else if c == '"' {
				return s, nil
			} else {
				s += string(c)
			}
		case 2:
			s += string(c)
			phase--
		}
	}
	return s, nil
}

func readMap(rd *bufio.Reader, typ string) (interface{}, os.Error) {
	var mii map[int]int
	var mis map[int]string
	var mi map[int]interface{}
	var msi map[string]int
	var mss map[string]string
	var ms map[string]interface{}
	var m interface{}
	b, _, e := rd.ReadRune()
	if e != nil {
		return nil, e
	}
	if b != '{' {
		return nil, os.NewError("map without open brace?")
	}
	switch typ {
	case "mii":
		mii = make(map[int]int)
		m = mii
	case "mis":
		mis = make(map[int]string)
		m = mis
	case "mi*":
		mi = make(map[int]interface{})
		m = mi
	case "msi":
		msi = make(map[string]int)
		m = msi
	case "mss":
		mss = make(map[string]string)
		m = mss
	case "ms*":
		ms = make(map[string]interface{})
		m = ms
	}
	for {
		b, e := rd.ReadByte()
		if e != nil {
			return nil, e
		}
		if b == '}' {
			break
		}
		rd.UnreadByte()
		k, e := _deserialize(rd)
		if e != nil {
			return nil, e
		}
		v, e := _deserialize(rd)
		if e != nil {
			return nil, e
		}
		switch typ {
		case "mii":
			mii[k.(int)] = v.(int)
		case "mis":
			mis[k.(int)] = v.(string)
		case "mi*":
			mi[k.(int)] = v
		case "msi":
			msi[k.(string)] = v.(int)
		case "mss":
			mss[k.(string)] = v.(string)
		case "ms*":
			ms[k.(string)] = v
		}
	}
	return m, nil
}

func readSlice(rd *bufio.Reader, typ string) (interface{}, os.Error) {
	var ai []int
	var as []string
	var a []interface{}
	var ret interface{}
	b, _, e := rd.ReadRune()
	if e != nil {
		return nil, e
	}
	if b != '{' {
		return nil, os.NewError("slice without open brace?")
	}
	n, e := strconv.Atoi(typ[2:])
	if e != nil {
		return nil, e
	}
	t := typ[1]
	switch t {
	case 'i':
		ai = make([]int, n)
		ret = ai
	case 's':
		as = make([]string, n)
		ret = as
	case '*':
		a = make([]interface{}, n)
		ret = a
	}
	for i := 0; i < n; i++ {
		e, _ := _deserialize(rd)
		switch t {
		case 'i':
			ai[i] = e.(int)
		case 's':
			as[i] = e.(string)
		case '*':
			a[i] = e
		}
	}
	b, _, e = rd.ReadRune()
	if e != nil {
		return nil, e
	}
	if b != '}' {
		return nil, os.NewError("slice without close brace?")
	}
	return ret, nil
}

func _deserialize(rd *bufio.Reader) (interface{}, os.Error) {
	var typ string
	var ret interface{}
	var e os.Error
	phase := 0
FOR: for {
		c, _, e := rd.ReadRune()
		if e != nil {
			if e == os.EOF {
				break
			}
			return nil, e
		}
		switch phase {
		case 0:
			if c == ':' {
				switch typ[0] {
				case 'i':
					ret, e = readInt(rd)
					phase = 1
				case 's':
					ret, e = readString(rd)
					phase = 1
				case 'a':
					ret, e = readSlice(rd, typ)
					break FOR
				case 'm':
					ret, e = readMap(rd, typ)
					break FOR
				case 't':
				default:
					return nil, os.NewError(fmt.Sprintf("type '%s' not supported", typ))
				}
			} else {
				typ += string(c)
			}
		case 1:
			if c == ';' {
				break FOR
			}
			return nil, os.NewError(fmt.Sprintf("type '%s' doesn't end with semicolon but %c", typ, c))
		}
	}
	return ret, e
}

func deserialize(filename string) (interface{}, os.Error) {
	file, e := os.Open(filename, os.O_RDONLY, 0)
	if e != nil {
		return nil, e
	}

	rd := bufio.NewReader(file)

	d, e := _deserialize(rd)
	file.Close()

	return d, e
}

type Session struct {
	sid  string
	data map[string]interface{}
}

var sessions = make(map[string]*Session)

func GetSession(c *Controller) *Session {
	var sid string
	LOAD: for {
		var ok bool
		sid, ok = c.Cookies["fastweb_sessid"]
		if !ok || len(sid) != 32 {
			break
		}
		for _, c := range sid {
			if !(c >= 'a' && c <= 'z' || c >= '0' && c <= '9') {
				break LOAD
			}
		}
		s, _ := sessions[sid]
		if s != nil {
			return s
		}

		d, e := deserialize(SessionFilePath + "/sess_" + sid)
		if e == nil && d != nil {
			s := &Session{
				sid:  sid,
				data: d.(map[string]interface{}),
			}
			return s
		}
		break
	}

	var uuid [16]byte

	for i := 0; i < 16; i++ {
		uuid[i] = byte(rand.Intn(255))
	}
	uuid[6] = (4 << 4) | (uuid[6] & 15)
	uuid[8] = (2 << 4) | (uuid[8] & 15)
	sid = fmt.Sprintf("%x%x%x%x%x", uuid[0:4], uuid[4:6], uuid[6:8], uuid[8:10], uuid[10:])

	s := &Session{
		sid:  sid,
		data: make(map[string]interface{}),
	}
	sessions[sid] = s

	c.SetCookieFull("fastweb_sessid", sid, nil, "", "", false, true)

	return s
}

func (s *Session) Set(key string, val interface{}) {
	s.data[key] = val
}

func (s *Session) Get(key string) (interface{}, bool) {
	v, b := s.data[key]
	return v, b
}

func (s *Session) GetInt(key string) (int, bool) {
	v, b := s.data[key]
	if !b {
		return 0, false
	}
	i, ok := v.(int)
	return i, ok
}

func (s *Session) GetString(key string) (string, bool) {
	v, b := s.data[key]
	if !b {
		return "", false
	}
	str, ok := v.(string)
	return str, ok
}

func (s *Session) GetMapStringString(key string) (map[string]string, bool) {
	v, b := s.data[key]
	if !b {
		return nil, false
	}
	m, ok := v.(map[string]string)
	return m, ok
}

func (s *Session) GetMapStringInt(key string) (map[string]int, bool) {
	v, b := s.data[key]
	if !b {
		return nil, false
	}
	m, ok := v.(map[string]int)
	return m, ok
}

func (s *Session) GetMapString(key string) (map[string]interface{}, bool) {
	v, b := s.data[key]
	if !b {
		return nil, false
	}
	m, ok := v.(map[string]interface{})
	return m, ok
}

func (s *Session) GetMapIntString(key string) (map[int]string, bool) {
	v, b := s.data[key]
	if !b {
		return nil, false
	}
	m, ok := v.(map[int]string)
	return m, ok
}

func (s *Session) GetMapIntInt(key string) (map[int]int, bool) {
	v, b := s.data[key]
	if !b {
		return nil, false
	}
	m, ok := v.(map[int]int)
	return m, ok
}

func (s *Session) GetMapInt(key string) (map[int]interface{}, bool) {
	v, b := s.data[key]
	if !b {
		return nil, false
	}
	m, ok := v.(map[int]interface{})
	return m, ok
}

func (s *Session) GetSliceString(key string) ([]string, bool) {
	v, b := s.data[key]
	if !b {
		return nil, false
	}
	sl, ok := v.([]string)
	return sl, ok
}

func (s *Session) GetSliceInt(key string) ([]int, bool) {
	v, b := s.data[key]
	if !b {
		return nil, false
	}
	sl, ok := v.([]int)
	return sl, ok
}

func (s *Session) GetSlice(key string) ([]interface{}, bool) {
	v, b := s.data[key]
	if !b {
		return nil, false
	}
	sl, ok := v.([]interface{})
	return sl, ok
}

func (s *Session) Close() os.Error {
	return serialize(SessionFilePath+"/sess_"+s.sid, 0600, s.data)
}
