// vim: set syntax=go autoindent:
// Copyright 2010 Ivan Wong. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
package main

import (
        "go-fastweb.googlecode.com/svn/trunk/src/fastweb"
        "os"
)

type Products struct {
	fastweb.Controller
	name string
	brand string
	features []string
	specifications []string
	image string
}

type TestStruct struct {
	name string
	brand string
	features []string
	specifications []string
	image string
}

func (p *Products) View(id string) os.Error {
	if id == "ah64" {
		p.name = "RC Apache AH64 4-Channel Electric Helicoper"
		p.brand = "Colco"
		p.features = []string{
			"4 channel radio control duel propeller system",
			"Full movement controll: forward, backward, left, right, up and down",
			"Replica design",
			"Revolutionary co-axial rotor technology",
		}
		p.specifications = []string{
			"Dimensions: L 16 Inches X W 5.5 Inches x H 6.5 Inches",
			"Battery Duration: 10 min",
			"Range: 120 Feet",
		}
		p.image = "/img/ah64.jpg"
	}
	/*p.StartSession()
	s := p.Session
	m := make(map[string]interface{})
	m["name"] = p.name
	m["brand"] = p.brand
	m["features"] = p.features
	m["specifications"] = p.specifications
	m["image"] = p.image
	s.Set("test", "string")
	s.Set("test2", m)*/
	return nil
}

func main() {
	a := fastweb.NewApplication()
	a.RegisterController(&Products{})
        a.Run(":12345")
}

