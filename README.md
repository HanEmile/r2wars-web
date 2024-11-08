# r2wars

## nix

So I use nix for all kinds of things nowadays and so here as well.
You don't have to if you don't want to and that's ok.

There's a nix package here:
https://git.emile.space/hefe/tree/nix/pkgs/r2wars-web/default.nix

And a nix module using that package here:
https://git.emile.space/hefe/tree/nix/modules/r2wars-web/default.nix

And the configuration I'm currently running on https://r2wa.rs using the module which uses the package here:
https://git.emile.space/hefe/tree/nix/hosts/corrino/www/r2wa.rs.nix

If you want to clone from git.emile.space, you can currently do so like this:

```
git clone git://git.emile.space/r2wars-web.git
git clone git://git.emile.space/hefe.git
```

## Usage

```
; CGO_ENABLED=0 SESSION_KEY=aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa go run ./src --help
Usage of /var/folders/bt/2db5y4ds5yq2y9m29tt8g5dm0000gn/T/go-build1055665732/b001/exe/src:
  -databasepath string
    	The path to the main database (default "./main.db")
  -h string
    	The host to listen on (shorthand) (default "127.0.0.1")
  -host string
    	The host to listen on (default "127.0.0.1")
  -logfilepath string
    	The path to the log file (default "./server.log")
  -p int
    	The port to listen on (shorthand) (default 8080)
  -port int
    	The port to listen on (default 8080)
  -sessiondbpath string
    	The path to the session database (default "./sesions.db")
  -templates string
    	The path to the templates used (default "./templates")
```

## Architecture

There are essentially the following objects which are all linked to each other (using a table joining their ids):

- User
  - You, the player
- Bots
  - The bots to be run within a battle
- Battles
  - An arena in which bots can be placed and run. Constraints on what bots can be added are defined here
- Architectures
  - The archs supported by r2 in order to be used by bots and battles
- Bits
  - The bits (8, 16, 32, 64) supported by r2 in order to be used by bots and battles

## TODO

- [ ] Add user creating battle as default owner
- [ ] Allow adding other users as owners to battles
- [ ] Implement submitting bots
- [ ] Implement running the battle
- [ ] Add a "start battle now" button
- [ ] Add a "battle starts at this time" field into the battle
- [ ] Figure out how time is stored and restored with the db
- [ ] Do some magic to display the current fight backlog with all info
- [ ] After having added a bot to a battle with the right arch, the arch can be changed
      When updating the bot, make sure that it is still valid in all currently linked battles
