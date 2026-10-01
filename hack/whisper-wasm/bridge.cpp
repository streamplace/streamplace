#include "whisper.h"
#include <emscripten/bind.h>
#include <algorithm>
#include <stdexcept>
#include <vector>

static whisper_context * context = nullptr;
static std::vector<float> pcm;

static bool load(const std::string & path) {
    if (context) whisper_free(context);
    auto params = whisper_context_default_params();
    params.use_gpu = false;
    context = whisper_init_from_file_with_params(path.c_str(), params);
    return context != nullptr;
}

// The JS caller writes directly into this buffer, avoiding a second PCM copy.
static uintptr_t audio_buffer(int samples) {
    pcm.resize(samples);
    return reinterpret_cast<uintptr_t>(pcm.data());
}

static emscripten::val transcribe(const std::string & language, int threads) {
    if (!context) throw std::runtime_error("Model is not loaded");
    auto params = whisper_full_default_params(WHISPER_SAMPLING_GREEDY);
    params.language = language.empty() ? "auto" : language.c_str();
    params.n_threads = std::clamp(threads, 1, 4);
    params.print_realtime = params.print_progress = params.print_timestamps = false;
    params.no_context = true;
    params.single_segment = false;
    if (whisper_full(context, params, pcm.data(), pcm.size()) != 0)
        throw std::runtime_error("Whisper transcription failed");
    auto result = emscripten::val::object();
    auto segments = emscripten::val::array();
    for (int i = 0; i < whisper_full_n_segments(context); ++i) {
        auto segment = emscripten::val::object();
        segment.set("text", std::string(whisper_full_get_segment_text(context, i)));
        segment.set("startMs", double(whisper_full_get_segment_t0(context, i) * 10));
        segment.set("endMs", double(whisper_full_get_segment_t1(context, i) * 10));
        segments.call<void>("push", segment);
    }
    result.set("segments", segments);
    result.set("language", std::string(whisper_lang_str(whisper_full_lang_id(context))));
    return result;
}

EMSCRIPTEN_BINDINGS(streamplace_caption_whisper) {
    emscripten::function("load", &load);
    emscripten::function("audioBuffer", &audio_buffer);
    emscripten::function("transcribe", &transcribe);
}
