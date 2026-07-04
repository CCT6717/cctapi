Status: DONE

Commit(s): 0b86e49 feat: translate responses streaming events

Tests:
- `$env:PATH='D:\ct\tools\go1.22.12\bin;D:\ct\tools\w64devkit-1.23.0\bin;' + $env:PATH; $env:CGO_ENABLED='1'; $env:GOMAXPROCS='1'; go test -p 1 ./relay/model -run 'Test(ChatCompletionStreamToResponsesEvents|WriteResponsesSSE)' -count=1` - PASS (`ok github.com/songquanpeng/one-api/relay/model`)
- `$env:PATH='D:\ct\tools\go1.22.12\bin;D:\ct\tools\w64devkit-1.23.0\bin;' + $env:PATH; $env:CGO_ENABLED='1'; $env:GOMAXPROCS='1'; go test -p 1 ./controller -run 'TestRelayResponsesStreamConvertsSSEAndUsesFinalHeadersOnly' -count=1` - PASS (`ok github.com/songquanpeng/one-api/controller`)
- `$env:PATH='D:\ct\tools\go1.22.12\bin;D:\ct\tools\w64devkit-1.23.0\bin;' + $env:PATH; $env:CGO_ENABLED='1'; $env:GOMAXPROCS='1'; go test -p 1 ./relay/model ./controller ./router ./relay/relaymode -count=1` - PASS (`ok github.com/songquanpeng/one-api/relay/model`, `ok github.com/songquanpeng/one-api/controller`, `ok github.com/songquanpeng/one-api/router`, `ok github.com/songquanpeng/one-api/relay/relaymode`)

Concerns: none
