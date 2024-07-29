build:
	@go build -o yt-chat main.go

run: build
	@./yt-chat

clean:
	rm yt-chat