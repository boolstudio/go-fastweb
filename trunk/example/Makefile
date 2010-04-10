include ../src/Make.$(GOARCH)

all: simple

clean:
	rm -f simple

%.${O}: %.go
	${GC} -I . $<

%: %.${O} ../src/fastweb/_obj/fastweb.a ../src/fastcgi/_obj/fastcgi.a
	${LD} -L . -o $@ $<

