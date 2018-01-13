#ifndef JUKYBOX_AUDIOPLAYER_OMX_H
#define JUKYBOX_AUDIOPLAYER_OMX_H

#include <ilclient.h>
#include <stdio.h>

typedef struct {
  ILCLIENT_T* handle;
  COMPONENT_T* decoder;
  COMPONENT_T* renderer;
  TUNNEL_T tunnel;
  int firstFrame;
} OMXClient;

typedef enum OMXClientEncoding {
  OMXClientEncoding_PCM = 0,
  OMXClientEncoding_DTS,
  OMXClientEncoding_DDP
} OMXClientEncoding;

OMXClient* OMXClient_Create();
int OMXClient_Write(OMXClient* client, const char* data, int len);
int OMXClient_Start(OMXClient* client, int numChannels, int bitsPerSample, int sampleRate, int isFloatPlanar, OMXClientEncoding codec);
void OMXClient_Stop(OMXClient* client);
void OMXClient_Destroy(OMXClient* client);

#endif
