all: libemail.so # libemailgo.so

# libemailgo.so: main.go formats.go utils.go libemailgo.go
#	go build -buildmode c-shared -o $@ $^

libemail.so: main.go formats.go utils.go libemail.go
	go build -buildmode c-shared -o $@ $^

clean:
	rm -f libemailgo.h libemailgo.so libemail.h libemail.so

# libemailgo.go: defaultlogo.png

