include $(GOROOT)/src/Make.inc

all: simple

clean:
	rm -f simple

%.${O}: %.go
	${GC} -I . $<

%: %.${O} ../src/fastweb/_obj/go-fastweb.googlecode.com/svn/trunk/src/fastweb.a
	${LD} -L ../src/fastweb/_obj:. -o $@ $<

