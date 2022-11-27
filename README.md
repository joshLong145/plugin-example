# Ignite CLI example plugin

This repository is an example implementation of a plugin for the Ignite CLI. This plugin provides `hooks` to resolve, and manage an `IPFS` local instance.

## Hooks
- `ignite chain serve` resolves and starts an IPPS instance
- `ignite chain build` resolves an IPFS instance

## Commands
- `ignite ipfs shutdown` stops the IPFS instance

supported platforms
- Linux (AMD, ARM)
- MacOS (AMD, ARM)