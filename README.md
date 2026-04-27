[English](/README.md) | [فارسی](/README.fa_IR.md) | [العربية](/README.ar_EG.md) |  [中文](/README.zh_CN.md) | [Español](/README.es_ES.md) | [Русский](/README.ru_RU.md)

<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="./media/3x-ui-dark.png">
    <img alt="3x-ui" src="./media/3x-ui-light.png">
  </picture>
</p>

[![Release](https://img.shields.io/github/v/release/Sora39831/3x-ui.svg)](https://github.com/Sora39831/3x-ui/releases)
[![Build](https://img.shields.io/github/actions/workflow/status/Sora39831/3x-ui/release.yml.svg)](https://github.com/Sora39831/3x-ui/actions)
[![GO Version](https://img.shields.io/github/go-mod/go-version/Sora39831/3x-ui.svg)](#)
[![Downloads](https://img.shields.io/github/downloads/Sora39831/3x-ui/total.svg)](https://github.com/Sora39831/3x-ui/releases/latest)
[![License](https://img.shields.io/badge/license-GPL%20V3-blue.svg?longCache=true)](https://www.gnu.org/licenses/gpl-3.0.en.html)
[![Go Reference](https://pkg.go.dev/badge/github.com/Sora39831/3x-ui/v2.svg)](https://pkg.go.dev/github.com/Sora39831/3x-ui/v2)
[![Go Report Card](https://goreportcard.com/badge/github.com/Sora39831/3x-ui/v2)](https://goreportcard.com/report/github.com/Sora39831/3x-ui/v2)

**3X-UI** — advanced, open-source web-based control panel designed for managing Xray-core server. It offers a user-friendly interface for configuring and monitoring various VPN and proxy protocols.

> [!IMPORTANT]
> This project is only for personal usage, please do not use it for illegal purposes, and please do not use it in a production environment.

As an enhanced fork of the original X-UI project, 3X-UI provides improved stability, broader protocol support, and additional features.

## Quick Start

```bash
bash <(curl -Ls https://raw.githubusercontent.com/Sora39831/3x-ui/master/install.sh)
```

For full documentation, please visit the [project Wiki](https://github.com/Sora39831/3x-ui/wiki).

## Building from source

Generate fingerprinted frontend assets before compiling:

```bash
go run ./cmd/genassets
go build -ldflags "-w -s" -o build/x-ui main.go
```

Production builds embed files from `web/public/assets` and `web/public/assets-manifest.json`.

## Multi-Node Shared Control

- use MariaDB as the shared control database
- keep one `master` node for shared-account writes
- configure other nodes as `worker`
- workers rebuild local Xray config from synchronized snapshots
- traffic is flushed back as deltas, not absolute totals

## A Special Thanks to

- [alireza0](https://github.com/alireza0/)

## Acknowledgment

## Support project

**If this project is helpful to you, you may wish to give it a**:star2:

<a href="https://www.buymeacoffee.com/MHSanaei" target="_blank">
<img src="./media/default-yellow.png" alt="Buy Me A Coffee" style="height: 70px !important;width: 277px !important;" >
</a>

</br>
<a href="https://nowpayments.io/donation/hsanaei" target="_blank" rel="noreferrer noopener">
   <img src="./media/donation-button-black.svg" alt="Crypto donation button by NOWPayments">
</a>

## Stargazers over Time

[![Stargazers over time](https://starchart.cc/Sora39831/3x-ui.svg?variant=adaptive)](https://starchart.cc/Sora39831/3x-ui)
