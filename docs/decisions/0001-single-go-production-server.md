---
title: Single Go Production Server
type: decision
status: accepted
date: 2026-05-01
decision: 1
---

# Single Go Production Server

## Context

The application needs an API, safe media serving, realtime endpoints, and a
single-page frontend, but targets simple local deployment.

## Decision

Use one standard-library Go `net/http` process in production to serve the API,
media, generated artifacts, realtime transports, and built Svelte SPA.

## Consequences

Deployment is one executable plus static/content directories. Development may
still use Vite separately. Backend features must fit the existing Go ownership
boundaries rather than introduce an independent service without a new decision.

