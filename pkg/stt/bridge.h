#include <whisper.h>
#include <stdint.h>
int sp_whisper_supported_cpu(void);
void sp_log_init(void);
void sp_log_begin(void);
char *sp_log_end(void);
struct whisper_context *sp_whisper_load(void *, size_t);
struct whisper_vad_context *sp_vad_load(void *, size_t, int);
int sp_whisper_run(struct whisper_context *, struct whisper_state *, const float *, int, int, const char *, const char *, uintptr_t);
