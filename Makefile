all:
	docker build -t barbuza/cw-monitor .
	docker run --rm -v "$(PWD)":/host barbuza/cw-monitor cp -v cw-monitor /host/cw-monitor
