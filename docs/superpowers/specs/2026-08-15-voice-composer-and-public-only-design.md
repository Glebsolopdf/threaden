# Voice Composer and Public-Only Runtime

## Goal

Use the right-side composer action as a microphone when the message is empty, use it as Send when text or attachments exist, and show a live recording surface with cancel in the center. Keep only the public web runtime enabled on the server.

## Design

`VoiceRecorder` owns microphone capture and exposes a normalized `audioLevel` signal driven by an `AnalyserNode`. `MessageComposerComponent` maps the signal to a small set of wave bars, renders the recording surface in place of the text input, and uses the same right-side button to stop and attach the resulting audio file. The centered cancel action calls the existing cancellation path and discards the pending blob.

The local `threaden-web.service` is stopped and disabled after the public web service is rebuilt and healthy. The public service remains on port 18082.

## Verification

Add unit coverage for the microphone level and composer state transitions. Run the full web test suite and production build, then check both systemd services and confirm the local web unit is disabled while the public unit is healthy.
