// vim: set syntax=go autoindent:
// Copyright 2010 Ivan Wong. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
package fastweb

import (
	//"dump"
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
		e := serialize(wr, v.Elem(i))
		if e != nil {
			return e
		}
	}
	_, e = io.WriteString(wr, "}")
	return e
}

func serialize(wr io.Writer, v reflect.Value) os.Error {
	var e os.Error
	switch vv := v.(type) {
	case *reflect.IntValue:
		_, e = io.WriteString(wr, "i:"+strconv.Itoa(vv.Get())+":")
	case *reflect.StringValue:
		_, e = io.WriteString(wr, "s:\""+escape(vv.Get())+"\":")
	case *reflect.SliceValue:
		var tstr string
		et := v.Type().(*reflect.SliceType).Elem()
		switch et.(type) {
		case *reflect.IntType:
			tstr = "i"
		case *reflect.StringType:
			tstr = "s"
		case *reflect.InterfaceType:
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
		default:
			return os.NewError("only map[]int, map[]string or map[]interface{} are supported")
		}
		_, e := io.WriteString(wr, "m"+kstr+estr+":{")
		if e != nil {
			return e
		}
		keys := vv.Keys()
		for _, k := range keys {
			e := serialize(wr, k)
			if e != nil {
				return e
			}
			e = serialize(wr, vv.Elem(k))
			if e != nil {
				return e
			}
		}
		_, e = io.WriteString(wr, "}")
	case *reflect.InterfaceValue:
		e = serialize(wr, vv.Elem())
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
			e := serialize(wr, reflect.NewValue(t.Field(i).Name))
			if e != nil {
				return e
			}
			e = serialize(wr, sv.Field(i))
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

func Serialize(filename string, perm int, data interface{}) os.Error {
	file, e := os.Open(filename, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, perm)
	if e != nil {
		return e
	}

	e = serialize(file, reflect.NewValue(data))
	file.Close()

	return e
}

func Deserialize(filename string) (interface{}, os.Error) {
	return nil, nil
}

type Session struct {
	sid  string
	data map[string]interface{}
}

var sessions = make(map[string]*Session)

func GetSession(c *Controller) *Session {
	sid, ok := c.Cookies["fastweb_sessid"]
	if ok {
		var invalid bool
		for _, c := range sid {
			if !(c >= 'a' && c <= 'z' || c >= '0' && c <= '9') {
				invalid = true
				break
			}
		}
		if !invalid {
			s, _ := sessions[sid]
			if s != nil {
				return s
			}

			d, e := Deserialize(SessionFilePath + "/sess_" + sid)
			if e == nil && d != nil {
				s := &Session{
					sid:  sid,
					data: d.(map[string]interface{}),
				}
				return s
			}
		}
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
	return Serialize(SessionFilePath + "/sess_" + s.sid, 0600, s.data)
}

