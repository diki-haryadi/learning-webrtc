# R&D on webrtc video streaming  
## Server : Pure Go WebRTC SFU [pion/ion-sfu](https://github.com/pion/ion-sfu)
## Frontend: React

![Demo](./demo.gif)

## Run the project
1. Start SFU Server with `go run main.go`
2. Start React Frontend with `cd client && yarn start`

## Implemented Features (till now)

* Audio / Video Streaming
* Screen Sharing
* Video/Audio Mute/UnMute
* Pin/UnPin
## Useful Articles

I will be adding the articles helpful for me in my R&D 

* [Building a WebRTC video and audio Broadcaster in Golang using ION-SFU, and media devices](https://gabrieltanner.org/blog/broadcasting-ion-sfu)

* [Custom ion-sfu Implementation Repo](https://github.com/dipeshdulal/custom-ion-sfu)

Thank you for the detailed instructions. Here is a summary of the steps you provided:

1. Go to the URL chrome://flags in your Chrome browser.
2. Search for the flag "Insecure origins treated as secure".
3. Change the setting for this flag to "Enabled".
4. In the provided textbox, enter the URL of the website you want to treat as secure.
5. Restart your Chrome browser.
This process must be done for each computer or browser you want to use to access the otherwise insecure website. However, you mentioned that this method worked well for you, which is good to know.
