include ../src/Make.$(GOARCH)

all: simple

clean:
	rm -f simple

%.${O}: %.go
	${GC} -I . $<

%: %.${O}
	${LD} -L . -o $@ $<

