---
title: Asteroids
type: reference
status: active
---

# Asteroids

Asteroids is a server-authoritative simulation in a fixed 1600x900 toroidal
arena. The server runs physics at 60 ticks per second and sends snapshots every
three ticks. The browser predicts only its local ship, extrapolates presentation
state, reconciles through `ackSeq`, and keeps particles and audio local.

## Presence And Rounds

Opening the site creates a spectator, but keyboard controls remain owned by the
page. An explicit click on the unobstructed page background arms Asteroids;
only then can the game keys start or control the local ship. Clicking any feed
card, panel, button, form control, dialog, or other application UI disarms game
controls. Restoring a server or session state never opens the local game before
that background click and a subsequent game key.

Player states are `spectator`, `active`, `inactive`, and `away`. A crash leaves
a player inactive in the same round; the next control respawns with collision
grace. Escape or disconnect keeps membership for a 10-second resume window.

Solo rounds last 60 seconds and save scores on the server. A finished round
stays finished while its player remains connected so the leaderboard remains
visible. Multiplayer uses server-owned kill counters and disables solo scoring.

## Commands And Authority

Commands are sequenced input, shoot, restart, finish, leave, name, and heartbeat
messages. The server owns collision, bullets, asteroids, score, respawn,
presence, and round transitions. Resume tokens reconnect the same player, and
connection leases prevent an older socket close from disconnecting a replacement.

## Power-Ups

The server spawns at most three pickups and owns collection and expiry:
stacking shields, timed triple shot, timed rapid fire with enforced cooldown,
timed overdrive, and nova. Nova destroys current asteroids and active enemy
ships, bypasses shields, and credits normal authoritative scores or kills.

Remote entities may render behind the feed for spectators. Background rendering
must not restart the dismissed local game or keep an idle animation loop alive.
