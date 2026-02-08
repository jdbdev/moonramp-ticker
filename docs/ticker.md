# Ticker Service

## Overview
Ticker service is an application level service layer that calls an external API to gather current cryptocurrency quotes and update the application persistence layer (database). It is called on a set time interval based on the API's query policies and desired token usage. 

## Responsibilities
- Fetch latest quotes for selected coins or tokens
- Decode and validate upstream responses from API
- Persist updated pricing data

## Architecture & flow
Ticker service is called by package main;

main -> TicerService -> external API -> CoinRepository

## Critical
- The service assumes upstream data is authoritative
- Idempotency is enforced at the persistence layer
