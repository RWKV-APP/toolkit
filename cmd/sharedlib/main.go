package main

/*
#include <stdint.h>
#include <stdlib.h>
*/
import "C"

import (
	"llm_toolkit"
	"unsafe"
)

func main() {}

func goString(input *C.char) string {
	if input == nil {
		return ""
	}
	return C.GoString(input)
}

func newCString(text string) *C.char {
	return C.CString(text)
}

//export LLMToolkit_GetHardwareInfo
func LLMToolkit_GetHardwareInfo(input *C.char) *C.char {
	return newCString(llm_toolkit.LLMToolkitGetHardwareInfoJSON(goString(input)))
}

//export LLMToolkit_GetHardwareUsageInfo
func LLMToolkit_GetHardwareUsageInfo(input *C.char) *C.char {
	return newCString(llm_toolkit.LLMToolkitGetHardwareUsageInfoJSON(goString(input)))
}

//export LLMToolkit_InitProvider
func LLMToolkit_InitProvider(input *C.char) *C.char {
	return newCString(llm_toolkit.LLMToolkitInitProviderJSON(goString(input)))
}

//export LLMToolkit_ListModels
func LLMToolkit_ListModels(input *C.char) *C.char {
	return newCString(llm_toolkit.LLMToolkitListModelsJSON(goString(input)))
}

//export LLMToolkit_SubmitChat
func LLMToolkit_SubmitChat(input *C.char) *C.char {
	return newCString(llm_toolkit.LLMToolkitSubmitChatJSON(goString(input)))
}

//export LLMToolkit_PollEvents
func LLMToolkit_PollEvents(input *C.char) *C.char {
	return newCString(llm_toolkit.LLMToolkitPollEventsJSON(goString(input)))
}

//export LLMToolkit_GetRequestState
func LLMToolkit_GetRequestState(input *C.char) *C.char {
	return newCString(llm_toolkit.LLMToolkitGetRequestStateJSON(goString(input)))
}

//export LLMToolkit_FreeCString
func LLMToolkit_FreeCString(ptr *C.char) {
	if ptr != nil {
		C.free(unsafe.Pointer(ptr))
	}
}
