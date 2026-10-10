#pragma once

#include <obs-module.h>

/* The output protocol the plugin registers. The services.json entry names it,
 * so OBS lists that entry only while the plugin is loaded. */
#define STREAMPLACE_PROTOCOL "Streamplace"

#define do_log(level, format, ...) blog(level, "[obs-streamplace] " format, ##__VA_ARGS__)

extern struct obs_output_info streamplace_output_info;
