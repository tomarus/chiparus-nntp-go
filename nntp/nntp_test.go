// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package nntp

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"strings"
	"testing"
	"time"
)

func TestSanityChecks(t *testing.T) {
	if _, err := Dial("", ""); err == nil {
		t.Fatal("Dial should require at least a destination address.")
	}
}

type faker struct {
	io.Writer
}

func (f faker) Close() error {
	return nil
}

func TestBasic(t *testing.T) {
	basicServer = strings.Join(strings.Split(basicServer, "\n"), "\r\n")
	basicClient = strings.Join(strings.Split(basicClient, "\n"), "\r\n")

	var cmdbuf bytes.Buffer
	var fake faker
	fake.Writer = &cmdbuf

	conn := &Conn{conn: fake, r: bufio.NewReader(strings.NewReader(basicServer))}

	// Test some global commands that don't take arguments
	if _, err := conn.Capabilities(); err != nil {
		t.Fatal("should be able to request CAPABILITIES after connecting: " + err.Error())
	}

	_, err := conn.Date()
	if err != nil {
		t.Fatal("should be able to send DATE: " + err.Error())
	}

	/*
		 Test broken until time.Parse adds this format.
		cdate := time.UTC()
		if sdate.Year != cdate.Year || sdate.Month != cdate.Month || sdate.Day != cdate.Day {
			t.Fatal("DATE seems off, probably erroneous: " + sdate.String())
		}
	*/

	// Test LIST (implicit ACTIVE)
	if _, err = conn.List(); err != nil {
		t.Fatal("LIST should work: " + err.Error())
	}

	loc, _ := time.LoadLocation("GMT")
	tt := time.Date(2010, 3, 1, 0, 0, 0, 0, loc)

	const grp = "gmane.comp.lang.go.general"
	_, l, h, err := conn.Group(grp)
	if err != nil {
		t.Fatal("Group shouldn't error: " + err.Error())
	}

	ov, err := conn.Over("1-")
	if err != nil {
		t.Fatal("Over shouldn't error: " + err.Error())
	}
	if ov[0].Subject != "Subject Data" {
		t.Fatal("Over returned mismatched subject")
	}

	hdr, err := conn.Hdr("Subject", "1-")
	if err != nil {
		t.Fatal("Hdr shouldn't error: " + err.Error())
	}
	if hdr[0].Header != "Subject Data" {
		t.Fatal("Hdr returned mismatched subject")
	}

	// test STAT, NEXT, and LAST
	if _, _, err = conn.Stat(""); err != nil {
		t.Fatal("should be able to STAT after selecting a group: " + err.Error())
	}
	if _, _, err = conn.Next(); err != nil {
		t.Fatal("should be able to NEXT after selecting a group: " + err.Error())
	}
	if _, _, err = conn.Last(); err != nil {
		t.Fatal("should be able to LAST after a NEXT selecting a group: " + err.Error())
	}

	// Can we grab articles?
	a, err := conn.Article(fmt.Sprintf("%d", l))
	if err != nil {
		t.Fatal("should be able to fetch the low article: " + err.Error())
	}
	body, err := ioutil.ReadAll(a.Body)
	if err != nil {
		t.Fatal("error reading reader: " + err.Error())
	}

	// Test that the article body doesn't get mangled.
	expectedbody := `Blah, blah.
.A single leading .
Fin.
`
	if !bytes.Equal([]byte(expectedbody), body) {
		t.Fatalf("article body read incorrectly; got:\n%s\nExpected:\n%s", body, expectedbody)
	}

	// Test articleReader
	expectedart := `Message-Id: <b@c.d>

Body.
`
	a, err = conn.Article(fmt.Sprintf("%d", l+1))
	if err != nil {
		t.Fatal("shouldn't error reading article low+1: " + err.Error())
	}
	var abuf bytes.Buffer
	_, err = a.WriteTo(&abuf)
	if err != nil {
		t.Fatal("shouldn't error writing out article: " + err.Error())
	}
	actualart := abuf.String()
	if actualart != expectedart {
		t.Fatalf("articleReader broke; got:\n%s\nExpected\n%s", actualart, expectedart)
	}

	// Just headers?
	if _, err = conn.Head(fmt.Sprintf("%d", h)); err != nil {
		t.Fatal("should be able to fetch the high article: " + err.Error())
	}

	// Without an id?
	if _, err = conn.Head(""); err != nil {
		t.Fatal("should be able to fetch the selected article without specifying an id: " + err.Error())
	}

	// How about bad articles? Do they error?
	if _, err = conn.Head(fmt.Sprintf("%d", l-1)); err == nil {
		t.Fatal("shouldn't be able to fetch articles lower than low")
	}
	if _, err = conn.Head(fmt.Sprintf("%d", h+1)); err == nil {
		t.Fatal("shouldn't be able to fetch articles higher than high")
	}

	// Just the body?
	r, err := conn.Body(fmt.Sprintf("%d", l))
	if err != nil {
		t.Fatal("should be able to fetch the low article body" + err.Error())
	}
	if _, err = ioutil.ReadAll(r); err != nil {
		t.Fatal("error reading reader: " + err.Error())
	}

	if _, err = conn.NewNews(grp, tt); err != nil {
		t.Fatal("newnews should work: " + err.Error())
	}

	// NewGroups
	if _, err = conn.NewGroups(tt); err != nil {
		t.Fatal("newgroups shouldn't error " + err.Error())
	}

	if err = conn.Quit(); err != nil {
		t.Fatal("Quit shouldn't error: " + err.Error())
	}

	actualcmds := cmdbuf.String()
	if basicClient != actualcmds {
		t.Fatalf("Got:\n%s\nExpected\n%s", actualcmds, basicClient)
	}
}

var basicServer = `101 Capability list:
VERSION 2
.
111 20100329034158
215 Blah blah
foo 7 3 y
bar 000008 02 m
.
211 100 1 100 gmane.comp.lang.go.general
224 Overview data
1	Subject Data	From <addr@fake>	Sun, 24 Jun 2012 00:00:00 +0200	<msg@id>	<ref@1> <ref@2>	1234	25	Xref: artnm gmane.comp.lang.go.general:1
.
221 Subject header data
1 Subject Data
.
223 1 <a@b.c> status
223 2 <b@c.d> Article retrieved
223 1 <a@b.c> Article retrieved
220 1 <a@b.c> article
Path: fake!not-for-mail
From: Someone
Newsgroups: gmane.comp.lang.go.general
Subject: [go-nuts] What about base members?
Message-ID: <a@b.c>

Blah, blah.
..A single leading .
Fin.
.
220 2 <b@c.d> article
Message-ID: <b@c.d>

Body.
.
221 100 <c@d.e> head
Path: fake!not-for-mail
Message-ID: <c@d.e>
.
221 100 <c@d.e> head
Path: fake!not-for-mail
Message-ID: <c@d.e>
.
423 Bad article number
423 Bad article number
222 1 <a@b.c> body
Blah, blah.
..A single leading .
Fin.
.
230 list of new articles by message-id follows
<d@e.c>
.
231 New newsgroups follow
.
205 Bye!
`

var basicClient = `CAPABILITIES
DATE
LIST
GROUP gmane.comp.lang.go.general
OVER 1-
HDR Subject 1-
STAT
NEXT
LAST
ARTICLE 1
ARTICLE 2
HEAD 100
HEAD
HEAD 0
HEAD 101
BODY 1
NEWNEWS gmane.comp.lang.go.general 20100301 000000 GMT
NEWGROUPS 20100301 000000 GMT
QUIT
`
