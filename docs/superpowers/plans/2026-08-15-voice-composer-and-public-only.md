# Voice Composer and Public-Only Runtime Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Replace the voice composer flow with a live waveform recorder and keep only the public web service enabled.

**Architecture:** Keep capture lifecycle in `VoiceRecorder`, add only audio-level sampling there, and keep presentation/state transitions in `MessageComposerComponent`. Reuse existing attachment validation and upload behavior. Runtime service changes are performed through `threadenctl` after the web build.

**Tech Stack:** Angular, TypeScript, MediaRecorder, Web Audio API, CSS, systemd.

## Global Constraints

- Empty composer: microphone action; non-empty composer: Send action.
- Recording: input is replaced by wave bars, right microphone stops, centered Cancel discards.
- Preserve nested scrolling and existing attachment quotas.
- Keep source files below 300 lines and directories at or below 5 source files.

---

### Task 1: Add recorder audio-level state

**Files:**
- Modify: `web-client/src/app/features/groups/attachments/voice/voice-recorder.ts`
- Test: `web-client/src/app/features/groups/attachments/voice/voice-recorder.spec.ts`

- [ ] Add a failing test asserting the recorder exposes a normalized audio level while recording.
- [ ] Run the focused test and verify it fails because no level is exposed.
- [ ] Add an analyser, a 60fps sampling loop, and cleanup for audio context/source/animation frame.
- [ ] Run the focused test and verify it passes.

### Task 2: Replace composer controls and render waves

**Files:**
- Modify: `web-client/src/app/features/groups/attachments/message-composer.component.ts`
- Modify: `web-client/src/styles/screens/groups/attachments/index.css`
- Test: `web-client/src/app/features/groups/attachments/voice/voice-recorder.spec.ts`

- [ ] Add tests for the public recorder stop/cancel transition used by the composer.
- [ ] Make the right action conditional: microphone for an empty composer, Send for body/files.
- [ ] Render recording waves and centered Cancel while recording; clicking the microphone calls stop.
- [ ] Add an accessible microphone icon and animated wave styling with reduced-motion support.
- [ ] Run all web tests and the production build.

### Task 3: Disable local runtime

**Files:**
- Runtime only: `threaden-web.service` via `systemctl`.

- [ ] Rebuild/restart backend, local web, and public web from the current checkout.
- [ ] Stop and disable `threaden-web.service`.
- [ ] Verify backend readiness, public web HTTP 200, and local web inactive/disabled.
