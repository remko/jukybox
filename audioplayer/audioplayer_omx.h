#ifndef JUKYBOX_AUDIOPLAYER_OMX_H
#define JUKYBOX_AUDIOPLAYER_OMX_H

#include <ilclient.h>
#include <stdio.h>

typedef struct {
	ILCLIENT_T* handle;
  COMPONENT_T* component;
} OMXClient;

OMXClient* OMXClient_Create();
int OMXClient_Write(OMXClient* client, const char* data, int len);
int OMXClient_Start(OMXClient* client, int numChannels, int bitsPerSample, int sampleRate, int isSideAndBackFlipped);
void OMXClient_Stop(OMXClient* client);
void OMXClient_Destroy(OMXClient* client);

#endif
