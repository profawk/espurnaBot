# Espurna Bot
A small bot for interaction with espurna

with this bot you can control an espurna switch remotely without exposing it to the internet

go check them out they are awesome! [espurna](https://github.com/xoseperez/espurna)

## Features
* Control the relay
* Schedule a task for the espurna (one time / recurring)*
* Nice interface and hopefully good UX 

## Installation

Install on this machine

```sh
go get -u github.com/profawk/espurnaBot
```

cross compile (for example for your 64bit Pi)

```sh
git clone https://github.com/profawk/espurnaBot
cd espurnaBot
CGO_ENABLED=0 GOARCH=arm64 go build
```

## Configuration steps

* Create a bot using @BotFather
* Get your chatID either from telegram (getUpdates) or from the bot's deny log
* Enable the http REST API on your espurna and get a key
* Make sure the espurna's ip is constat or resolvable
* Place your bot token, chat id, espurna ip, and API key in the config file
* Enjoy ;)

## Configuration file breakdown
* botToken - telegram bot access token
* chatIds - array of chatids allowed to access the bot
* watchdog - whether or not to check every minute if the relay state has changed externally
* espurna
  * relay - relay id which we control
  * hostname - espurna's accessible address
  * apiKey - key for REST API
  
## Plans
* Implement scheduling REST API for the espurna for unified tasks
* Allow full customization of all messages sent by bot (personalisation and i18n)
* Create a triggers system  e.g. alert the user 30 minutes after the water heater has been turned on and turn it off

## Notes
* \* the scheduling is done externally to the espurna for a lack of fitting API
