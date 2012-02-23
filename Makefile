all:
	$(MAKE) -C src
	$(MAKE) -C example

install:
	$(MAKE) -C src install

clean:
	$(MAKE) -C src clean
	$(MAKE) -C example clean

