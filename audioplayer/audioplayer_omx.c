// +build arm

#include "audioplayer_omx.h"
#include <assert.h>

#define BUFFER_SIZE_SAMPLES 1024
#define OUT_CHANNELS(n) ((n) > 4 ? 8: (n) > 2 ? 4: (n))
#define MIN(X, Y) (((X) < (Y)) ? (X) : (Y))

typedef int int32_t;

void handleError(void *userdata, COMPONENT_T *comp, OMX_U32 err) {
  fprintf(stderr, "omx error: ds\n", err);
}

void handleEOS(void *userdata, COMPONENT_T *comp, OMX_U32 data) {
  fprintf(stderr, "eos\n");
}

OMXClient* OMXClient_Create() {
  OMX_ERRORTYPE omxErr;

  OMXClient* client = malloc(sizeof(OMXClient));

  /////////////////////////////////////////////////////////////////////////
  // Initialize client
  /////////////////////////////////////////////////////////////////////////
  
  client->handle = ilclient_init();
  assert(client->handle != NULL);

  omxErr = OMX_Init();
  assert(omxErr == OMX_ErrorNone);

  ilclient_set_error_callback(client->handle, handleError, NULL);
  ilclient_set_eos_callback(client->handle, handleEOS, NULL);

  ilclient_create_component(client->handle, &client->component, "audio_render", ILCLIENT_DISABLE_ALL_PORTS | ILCLIENT_ENABLE_INPUT_BUFFERS);
  assert(client->component != NULL);

  return client;
}

int OMXClient_Start(OMXClient* client, int numChannels, int bitDepth, int sampleRate, int isSideAndBackFlipped) {
  OMX_ERRORTYPE omxErr;

  uint32_t bufferSize = (BUFFER_SIZE_SAMPLES * bitDepth * OUT_CHANNELS(numChannels))>>3;;
  uint32_t numBuffers = 10;

  int alignedBufferSize = (bufferSize + 15) & ~15;

  // Set format
  OMX_AUDIO_PARAM_PORTFORMATTYPE portFormat;
  memset(&portFormat, 0, sizeof(OMX_AUDIO_PARAM_PORTFORMATTYPE));
  portFormat.nSize = sizeof(OMX_AUDIO_PARAM_PORTFORMATTYPE);
  portFormat.nVersion.nVersion = OMX_VERSION;
  portFormat.nPortIndex = 100;
  omxErr = OMX_GetParameter(ilclient_get_handle(client->component), OMX_IndexParamAudioPortFormat, &portFormat);
  assert(omxErr == OMX_ErrorNone);
  portFormat.eEncoding = OMX_AUDIO_CodingPCM;
  omxErr = OMX_SetParameter(ilclient_get_handle(client->component), OMX_IndexParamAudioPortFormat, &portFormat);
  assert(omxErr == OMX_ErrorNone);

  // Initialize buffers
  OMX_PARAM_PORTDEFINITIONTYPE portDefinition;
  memset(&portDefinition, 0, sizeof(OMX_PARAM_PORTDEFINITIONTYPE));
  portDefinition.nSize = sizeof(OMX_PARAM_PORTDEFINITIONTYPE);
  portDefinition.nVersion.nVersion = OMX_VERSION;
  portDefinition.nPortIndex = 100;
  omxErr = OMX_GetParameter(ILC_GET_HANDLE(client->component), OMX_IndexParamPortDefinition, &portDefinition);
  assert(omxErr == OMX_ErrorNone);
  portDefinition.nBufferSize = alignedBufferSize;
  portDefinition.nBufferCountActual = numBuffers;
  omxErr = OMX_SetParameter(ILC_GET_HANDLE(client->component), OMX_IndexParamPortDefinition, &portDefinition);
  assert(omxErr == OMX_ErrorNone);

 // Initialize PCM parameters
  OMX_AUDIO_PARAM_PCMMODETYPE pcm;
  memset(&pcm, 0, sizeof(OMX_AUDIO_PARAM_PCMMODETYPE));
  pcm.nSize = sizeof(OMX_AUDIO_PARAM_PCMMODETYPE);
  pcm.nVersion.nVersion = OMX_VERSION;
  pcm.nPortIndex = 100;
  pcm.nChannels = OUT_CHANNELS(numChannels);
  pcm.eNumData = OMX_NumericalDataSigned;
  pcm.eEndian = OMX_EndianLittle;
  pcm.nSamplingRate = sampleRate;
  pcm.bInterleaved = OMX_TRUE;
  pcm.nBitPerSample = bitDepth;
  pcm.ePCMMode = OMX_AUDIO_PCMModeLinear;
  switch(numChannels) {
    case 1:
      pcm.eChannelMapping[0] = OMX_AUDIO_ChannelCF;
      break;
    case 3:
      pcm.eChannelMapping[2] = OMX_AUDIO_ChannelCF;
      pcm.eChannelMapping[1] = OMX_AUDIO_ChannelRF;
      pcm.eChannelMapping[0] = OMX_AUDIO_ChannelLF;
      break;
    case 8:
      pcm.eChannelMapping[7] = OMX_AUDIO_ChannelRS;
    case 7:
      pcm.eChannelMapping[6] = OMX_AUDIO_ChannelLS;
    case 6:
      pcm.eChannelMapping[5] = OMX_AUDIO_ChannelRR;
    case 5:
      pcm.eChannelMapping[4] = OMX_AUDIO_ChannelLR;
    case 4:
      pcm.eChannelMapping[3] = OMX_AUDIO_ChannelLFE;
      pcm.eChannelMapping[2] = OMX_AUDIO_ChannelCF;
    case 2:
      pcm.eChannelMapping[1] = OMX_AUDIO_ChannelRF;
      pcm.eChannelMapping[0] = OMX_AUDIO_ChannelLF;
      break;
  }
  if (isSideAndBackFlipped != 0) {
    OMX_AUDIO_CHANNELTYPE c7 = pcm.eChannelMapping[7];
    OMX_AUDIO_CHANNELTYPE c6 = pcm.eChannelMapping[6];
    pcm.eChannelMapping[7] = pcm.eChannelMapping[5];
    pcm.eChannelMapping[6] = pcm.eChannelMapping[4];
    pcm.eChannelMapping[5] = c7;
    pcm.eChannelMapping[4] = c6;
  }
  omxErr = OMX_SetParameter(ILC_GET_HANDLE(client->component), OMX_IndexParamAudioPcm, &pcm);
  assert(omxErr == OMX_ErrorNone);

  ilclient_change_component_state(client->component, OMX_StateIdle);

  // Set destination
  OMX_CONFIG_BRCMAUDIODESTINATIONTYPE destination;
  memset(&destination, 0, sizeof(destination));
  destination.nSize = sizeof(OMX_CONFIG_BRCMAUDIODESTINATIONTYPE);
  destination.nVersion.nVersion = OMX_VERSION;
  strcpy((char *)destination.sName, "hdmi");
  omxErr = OMX_SetConfig(ILC_GET_HANDLE(client->component), OMX_IndexConfigBrcmAudioDestination, &destination);
  assert(omxErr == OMX_ErrorNone);

  // Enable buffers
  int err = ilclient_enable_port_buffers(client->component, 100, NULL, NULL, NULL);
  assert(err == 0);

  ilclient_change_component_state(client->component, OMX_StateExecuting);

  return 0;
}

void OMXClient_Stop(OMXClient* client) {
  ilclient_change_component_state(client->component, OMX_StateIdle);

  ilclient_disable_port_buffers(client->component, 100, NULL, NULL, NULL);
    
  ilclient_change_component_state(client->component, OMX_StateLoaded);
}

void OMXClient_Destroy(OMXClient* client) {
  OMX_ERRORTYPE omxErr;

  COMPONENT_T* components[1];
  components[0] = client->component;
  ilclient_cleanup_components(components);

  omxErr = OMX_Deinit();
  ilclient_destroy(client->handle);
  assert(omxErr == OMX_ErrorNone);

  free(client);
}

int OMXClient_Write(OMXClient* client, const char* data, int len) {
  while (len > 0) {
    OMX_BUFFERHEADERTYPE* buffer = ilclient_get_input_buffer(client->component, 100, 1);
    int numWrite = MIN(buffer->nAllocLen, len);
    memcpy(buffer->pBuffer, data, numWrite);
    buffer->nFilledLen = numWrite;
    data += numWrite;
    len -= numWrite;
    OMX_ERRORTYPE err = OMX_EmptyThisBuffer(ilclient_get_handle(client->component), buffer);
    assert(err == OMX_ErrorNone);
  }
  return 0;
}
