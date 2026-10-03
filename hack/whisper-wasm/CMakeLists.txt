cmake_minimum_required(VERSION 3.16)
project(streamplace_caption_whisper C CXX)
set(CMAKE_CXX_STANDARD 17)
set(BUILD_SHARED_LIBS OFF CACHE BOOL "" FORCE)
set(WHISPER_BUILD_TESTS OFF CACHE BOOL "" FORCE)
set(WHISPER_BUILD_EXAMPLES OFF CACHE BOOL "" FORCE)
set(WHISPER_BUILD_SERVER OFF CACHE BOOL "" FORCE)
set(GGML_NATIVE OFF CACHE BOOL "" FORCE)
set(GGML_OPENMP OFF CACHE BOOL "" FORCE)
set(GGML_BLAS OFF CACHE BOOL "" FORCE)
set(GGML_METAL OFF CACHE BOOL "" FORCE)
# Do not accidentally stamp the enclosing Streamplace checkout's git revision
# into sources extracted from a pinned tarball.
set(GIT_EXE OFF CACHE FILEPATH "" FORCE)
set(GGML_BUILD_COMMIT "4979e04f5dcaccb36057e059bbaed8a2f5288315")
set(WHISPER_BUILD_COMMIT "4979e04f5dcaccb36057e059bbaed8a2f5288315")
set(WHISPER_BUILD_NUMBER 0)
set(CMAKE_C_FLAGS "${CMAKE_C_FLAGS} -pthread -msimd128")
set(CMAKE_CXX_FLAGS "${CMAKE_CXX_FLAGS} -pthread -msimd128")
add_subdirectory("${WHISPER_SOURCE}" whisper)
add_executable(caption-whisper bridge.cpp)
target_link_libraries(caption-whisper PRIVATE whisper)
set_target_properties(caption-whisper PROPERTIES SUFFIX ".mjs")
target_link_options(caption-whisper PRIVATE --bind -pthread -msimd128
  "-sMODULARIZE=1" "-sEXPORT_ES6=1" "-sEXPORT_NAME=createCaptionWhisper"
  "-sENVIRONMENT=web,worker" "-sPTHREAD_POOL_SIZE=4"
  "-sINITIAL_MEMORY=268435456" "-sMAXIMUM_MEMORY=2147483648"
  "-sALLOW_MEMORY_GROWTH=1" "-sSTACK_SIZE=5242880"
  "-sEXPORTED_RUNTIME_METHODS=['FS','HEAPF32']")
