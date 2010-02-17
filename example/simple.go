// vim: set syntax=go autoindent:
// Copyright 2010 Ivan Wong. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
package main

import (
        "fastweb"
        "fmt"
        "os"
)

type Test struct {
	fastweb.Controller
	a int
}

func (t *Test) DefaultAction() string {
	return "test"
}

func (t *Test) Test(id int, user string, passwd string) os.Error {
	fmt.Printf("%d %s %s\n", id, user, passwd)
	return nil
}

func main() {
	a := fastweb.NewApplication()
	t := &Test{a:1}
	a.RegisterController(t)
        a.Run(":12345")
}

