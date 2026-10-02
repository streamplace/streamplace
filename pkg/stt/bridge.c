#include "bridge.h"
#include <string.h>
#if defined(__x86_64__) || defined(_M_X64)
#include <cpuid.h>
#endif
int sp_whisper_supported_cpu(void) {
#if defined(__x86_64__) || defined(_M_X64)
    // CPUID directly: __builtin_cpu_supports needs a runtime osxcross lacks,
    // and GCC before 11 (the Windows cross compiler) can't test f16c with it.
    unsigned a, b, c, d, xcr0;
    if (!__get_cpuid(1, &a, &b, &c, &d) || !(c & bit_FMA) || !(c & bit_F16C) || !(c & bit_OSXSAVE)) {
        return 0;
    }
    // AVX registers are usable only if the OS saves them (XCR0 bits 1 and 2).
    __asm__("xgetbv" : "=a"(xcr0), "=d"(d) : "c"(0));
    return (xcr0 & 6) == 6 && __get_cpuid_count(7, 0, &a, &b, &c, &d) && (b & bit_AVX2);
#else
    return 1;
#endif
}
extern int goWhisperAbort(uintptr_t);
static bool abort_call(void *p) { return goWhisperAbort((uintptr_t)p) != 0; }
struct whisper_context *sp_whisper_load(void *data, size_t len) {
    struct whisper_context_params p = whisper_context_default_params();
    p.use_gpu = false;
    return whisper_init_from_buffer_with_params_no_state(data, len, p);
}
struct buffer { const char *data; size_t len, pos; };
static size_t buffer_read(void *ptr, void *out, size_t n) {
    struct buffer *b = ptr;
    if (n > b->len - b->pos) n = b->len - b->pos;
    memcpy(out, b->data + b->pos, n); b->pos += n; return n;
}
static bool buffer_eof(void *ptr) { struct buffer *b = ptr; return b->pos >= b->len; }
static void buffer_close(void *ptr) { (void)ptr; }
struct whisper_vad_context *sp_vad_load(void *data, size_t len, int threads) {
    struct buffer b = {data, len, 0};
    struct whisper_model_loader loader = {&b, buffer_read, buffer_eof, buffer_close};
    struct whisper_vad_context_params p = whisper_vad_default_context_params();
    p.use_gpu = false; p.n_threads = threads;
    return whisper_vad_init_with_params(&loader, p);
}
int sp_whisper_run(struct whisper_context *ctx, struct whisper_state *state,
        const float *pcm, int n, int threads, const char *language, const char *prompt, uintptr_t handle) {
    struct whisper_full_params p = whisper_full_default_params(WHISPER_SAMPLING_GREEDY);
    p.n_threads = threads; p.language = language; p.initial_prompt = prompt;
    p.no_context = true; p.carry_initial_prompt = true;
    p.token_timestamps = true; p.split_on_word = true;
    p.print_progress = false; p.print_realtime = false; p.print_timestamps = false;
    p.temperature_inc = 0; p.greedy.best_of = 1;
    p.abort_callback = abort_call; p.abort_callback_user_data = (void *)handle;
    return whisper_full_with_state(ctx, state, p, pcm, n);
}
