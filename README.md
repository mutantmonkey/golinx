# golinx

A client for [linx-server](https://github.com/andreimarcu/linx-server) written
in Go.

## Configuration
Golinx follows the XDG directory standard, so you should put your config in
`~/.config/golinx/config.yml`, or simply specify the path to it with the
`-config` flag.

Here's an example config file:
```
server: http://example.com
proxy: socks5://127.0.0.1:9050
uploadlog: /home/mutantmonkey/.local/share/golinx/linx.log
```

## License
Copyright (c) 2015-2019 mutantmonkey

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with this program.  If not, see <http://www.gnu.org/licenses/>.
