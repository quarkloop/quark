# JSON Pipeline Example

A simple pipeline that demonstrates the new node architecture:

```
timer → cpu-profiler → field-mapper → stdout-logger
                    ↘ file-writer
```

## Nodes used

| Node | URI | Language |
|------|-----|----------|
| Timer | `quark/time/schedule/timer:v1` | Java |
| CPU Profiler | `quark/system/cpu/profile:v1` | Java |
| Field Mapper | `quark/data/shape/map:v1` | TypeScript |
| Stdout Logger | `quark/log/console/stdout:v1` | TypeScript |
| File Writer | `quark/io/file/write:v1` | Java |

## Deploy

```bash
quarkctl apply -f system.quark.ts -n demo
```

## Verify

```bash
# Watch the stdout output in the data plane log
tail -f $QUARK_STATE_ROOT/dataplane-logs/dataplane-shared.log

# Check the file output
cat example/json-pipeline/output.jsonl
```

## Undeploy

```bash
quarkctl delete system json-pipeline -n demo
```
