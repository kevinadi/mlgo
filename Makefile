version=$(shell git describe --always --long --dirty)
date=$(shell date -j "+(%b %Y)")
exec=mlgo

all:
	@go build -v -ldflags '-X "main.version=${version}" -X "main.date=${date}"'

install:
	@go install -v -ldflags '-X "main.version=${version}" -X "main.date=${date}"'

clean:
	@rm -f ${exec}
