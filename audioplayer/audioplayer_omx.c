// +build arm

#include "audioplayer_omx.h"
#include <assert.h>
#include "wav.h"

#define BUFFER_SIZE_SAMPLES 1024
#define MIN(X, Y) (((X) < (Y)) ? (X) : (Y))
#define MAX(X, Y) (((X) > (Y)) ? (X) : (Y))

typedef int int32_t;

static const char OUT_CHANNELS[] = { 1, 1, 2, 4, 4, 8, 8, 8, 8};

void handleError(void *userdata, COMPONENT_T *comp, OMX_U32 err) {
  if (err == OMX_ErrorSameState) {
    return;
  }
  fprintf(stderr, "omx error: %x\n", err);
}

void handleEOS(void *userdata, COMPONENT_T *comp, OMX_U32 data) {
  fprintf(stderr, "eos\n");
}

/*
void handleBufferEmptied(void *userdata, COMPONENT_T *comp) {
  fprintf(stderr, "buffer emptied\n");
}

void handleBufferFilled(void *userdata, COMPONENT_T *comp) {
  fprintf(stderr, "buffer filled\n");
}
*/

void printPCM(COMPONENT_T* comp, int portIndex) {
  OMX_AUDIO_PARAM_PCMMODETYPE pcm;
  memset(&pcm, 0, sizeof(OMX_AUDIO_PARAM_PCMMODETYPE));
  pcm.nSize = sizeof(OMX_AUDIO_PARAM_PCMMODETYPE);
  pcm.nVersion.nVersion = OMX_VERSION;
  pcm.nPortIndex = portIndex;
  OMX_ERRORTYPE omxErr = OMX_GetParameter(ILC_GET_HANDLE(comp), OMX_IndexParamAudioPcm, &pcm);
  if (omxErr == OMX_ErrorNone) {
    printf("PCM %d: %d %d %d %d\n", portIndex, pcm.nChannels, pcm.nSamplingRate, pcm.nBitPerSample, pcm.ePCMMode);
  } else {
    printf("Passthrough\n");
  }
}


OMXClient* OMXClient_Create() {
  OMX_ERRORTYPE omxErr;

  OMXClient* client = malloc(sizeof(OMXClient));

  client->handle = ilclient_init();
  assert(client->handle != NULL);

  omxErr = OMX_Init();
  assert(omxErr == OMX_ErrorNone);

  ilclient_set_error_callback(client->handle, handleError, NULL);
  ilclient_set_eos_callback(client->handle, handleEOS, NULL);
  /* ilclient_set_empty_buffer_done_callback(client->handle, handleBufferEmptied, NULL); */
  /* ilclient_set_fill_buffer_done_callback(client->handle, handleBufferFilled, NULL); */

  ilclient_create_component(client->handle, &client->decoder, "audio_decode", ILCLIENT_DISABLE_ALL_PORTS | ILCLIENT_ENABLE_INPUT_BUFFERS);
  assert(client->decoder != NULL);

  ilclient_create_component(client->handle, &client->renderer, "audio_render", ILCLIENT_DISABLE_ALL_PORTS | ILCLIENT_ENABLE_INPUT_BUFFERS);
  assert(client->renderer != NULL);

  set_tunnel(&client->tunnel, client->decoder, 121, client->renderer, 100);

  return client;
}

void setupDecoderRendererTunnel(OMXClient* client) {
  int err = ilclient_setup_tunnel(&client->tunnel, 0, 0);
  assert(err == 0);

  OMX_ERRORTYPE omxErr = ilclient_change_component_state(client->renderer, OMX_StateExecuting);
  assert(omxErr == OMX_ErrorNone);

  printf("New port config: "); printPCM(client->decoder, 121);
}

int OMXClient_Start(OMXClient* client, int numChannels, int bitDepth, int sampleRate, int isFloatPlanar, OMXClientEncoding encoding) {
  OMX_ERRORTYPE omxErr;

  uint32_t bufferSize = (BUFFER_SIZE_SAMPLES * bitDepth * numChannels)>>3;;
  uint32_t numBuffers = 10;
  uint32_t bytesPerSec = sampleRate * (bitDepth >> 3) * OUT_CHANNELS[numChannels];
  uint32_t audioBufferSeconds = 3;

  int alignedBufferSize = (bufferSize + 15) & ~15;

  OMX_AUDIO_CODINGTYPE omxEncoding = OMX_AUDIO_CodingPCM;
  switch (encoding) {
    case OMXClientEncoding_DTS: omxEncoding = OMX_AUDIO_CodingDTS; break;
    case OMXClientEncoding_DDP: omxEncoding = OMX_AUDIO_CodingDDP; break;
  }

  if (omxEncoding != OMX_AUDIO_CodingPCM) {
    OMX_CONFIG_BOOLEANTYPE passthrough;
    passthrough.nSize = sizeof(OMX_CONFIG_BOOLEANTYPE);
    passthrough.nVersion.nVersion = OMX_VERSION;
    passthrough.bEnabled = OMX_TRUE;
    omxErr = OMX_SetParameter(ilclient_get_handle(client->decoder), OMX_IndexParamBrcmDecoderPassThrough, &passthrough);
    assert(omxErr == OMX_ErrorNone);
  }

  // Initialize buffers
  OMX_PARAM_PORTDEFINITIONTYPE portDefinition;
  memset(&portDefinition, 0, sizeof(OMX_PARAM_PORTDEFINITIONTYPE));
  portDefinition.nSize = sizeof(OMX_PARAM_PORTDEFINITIONTYPE);
  portDefinition.nVersion.nVersion = OMX_VERSION;
  portDefinition.nPortIndex = 120;
  omxErr = OMX_GetParameter(ILC_GET_HANDLE(client->decoder), OMX_IndexParamPortDefinition, &portDefinition);
  assert(omxErr == OMX_ErrorNone);
  portDefinition.nBufferSize = alignedBufferSize;
  portDefinition.nBufferCountActual = numBuffers;
  portDefinition.format.audio.eEncoding = omxEncoding;
  omxErr = OMX_SetParameter(ILC_GET_HANDLE(client->decoder), OMX_IndexParamPortDefinition, &portDefinition);
  assert(omxErr == OMX_ErrorNone);

  //portDefinition.nBufferCountActual = MAX(portDefinition.nBufferCountMin, (bytesPerSec * audioBufferSeconds) / portDefinition.nBufferSize);
  //omxErr = OMX_SetParameter(ILC_GET_HANDLE(client->decoder), OMX_IndexParamPortDefinition, &portDefinition);
  //assert(omxErr == OMX_ErrorNone);

  ilclient_change_component_state(client->decoder, OMX_StateIdle);

  // Set format
  OMX_AUDIO_PARAM_PORTFORMATTYPE portFormat;
  memset(&portFormat, 0, sizeof(OMX_AUDIO_PARAM_PORTFORMATTYPE));
  portFormat.nSize = sizeof(OMX_AUDIO_PARAM_PORTFORMATTYPE);
  portFormat.nVersion.nVersion = OMX_VERSION;
  portFormat.nPortIndex = 120;
  /* omxErr = OMX_GetParameter(ilclient_get_handle(client->decoder), OMX_IndexParamAudioPortFormat, &portFormat); */
  /* assert(omxErr == OMX_ErrorNone); */
  portFormat.eEncoding = omxEncoding;
  omxErr = OMX_SetParameter(ilclient_get_handle(client->decoder), OMX_IndexParamAudioPortFormat, &portFormat);
  assert(omxErr == OMX_ErrorNone);

  // Enable buffers
  int err = ilclient_enable_port_buffers(client->decoder, 120, NULL, NULL, NULL);
  assert(err == 0);

  omxErr = ilclient_change_component_state(client->decoder, OMX_StateExecuting);
  assert(omxErr == OMX_ErrorNone);

  // Write configuration header
  if (omxEncoding == OMX_AUDIO_CodingPCM) {
    WAVEFORMATEXTENSIBLE waveHeader;
    memset(&waveHeader, 0x0, sizeof(waveHeader));
    waveHeader.Format.nChannels  = numChannels;
    waveHeader.dwChannelMask = SPEAKER_FRONT_LEFT | SPEAKER_FRONT_RIGHT;
    waveHeader.Samples.wSamplesPerBlock = 0;
    waveHeader.Format.nBlockAlign = numChannels * (bitDepth >> 3);
    if (isFloatPlanar) {
      waveHeader.Format.wFormatTag = 0x8000;
    }
    else {
      waveHeader.Format.wFormatTag = WAVE_FORMAT_PCM;
    }
    waveHeader.Format.nSamplesPerSec = sampleRate;
    waveHeader.Format.nAvgBytesPerSec = bytesPerSec;
    waveHeader.Format.wBitsPerSample = bitDepth;
    waveHeader.Samples.wValidBitsPerSample = bitDepth;
    waveHeader.Format.cbSize = 0;
    waveHeader.SubFormat = KSDATAFORMAT_SUBTYPE_PCM;

    OMX_BUFFERHEADERTYPE *buffer = ilclient_get_input_buffer(client->decoder, 120, 1);
    buffer->nOffset = 0;
    buffer->nFilledLen  = MIN(sizeof(waveHeader), buffer->nAllocLen);
    memset((unsigned char *)buffer->pBuffer, 0x0, buffer->nAllocLen);
    memcpy((unsigned char *)buffer->pBuffer, &waveHeader, buffer->nFilledLen);
    buffer->nFlags = OMX_BUFFERFLAG_CODECCONFIG | OMX_BUFFERFLAG_ENDOFFRAME;
    omxErr = OMX_EmptyThisBuffer(ilclient_get_handle(client->decoder), buffer);
    assert(omxErr == OMX_ErrorNone);
  }

  // Set renderer destination
  OMX_CONFIG_BRCMAUDIODESTINATIONTYPE destination;
  memset(&destination, 0, sizeof(destination));
  destination.nSize = sizeof(OMX_CONFIG_BRCMAUDIODESTINATIONTYPE);
  destination.nVersion.nVersion = OMX_VERSION;
  strcpy((char *)destination.sName, "hdmi");
  omxErr = OMX_SetConfig(ILC_GET_HANDLE(client->renderer), OMX_IndexConfigBrcmAudioDestination, &destination);
  assert(omxErr == OMX_ErrorNone);

  client->firstFrame = 1;

  setupDecoderRendererTunnel(client);

  return 0;
}

void OMXClient_Stop(OMXClient* client) {
  ilclient_disable_tunnel(&client->tunnel);
  ilclient_change_component_state(client->decoder, OMX_StateIdle);

  ilclient_disable_port_buffers(client->decoder, 120, NULL, NULL, NULL);
    
  ilclient_change_component_state(client->decoder, OMX_StateLoaded);

  ilclient_change_component_state(client->renderer, OMX_StateLoaded);
}

void OMXClient_Destroy(OMXClient* client) {
  OMX_ERRORTYPE omxErr;

  COMPONENT_T* components[2];
  components[0] = client->decoder;
  components[1] = client->renderer;
  ilclient_cleanup_components(components);

  omxErr = OMX_Deinit();

  ilclient_destroy(client->handle);
  assert(omxErr == OMX_ErrorNone);

  free(client);
}

int OMXClient_Write(OMXClient* client, const char* data, int len) {
  while (len > 0) {
    OMX_BUFFERHEADERTYPE* buffer = ilclient_get_input_buffer(client->decoder, 120, 1);
    int numWrite = MIN(buffer->nAllocLen, len);
    memcpy(buffer->pBuffer, data, numWrite);
    buffer->nFilledLen = numWrite;
    data += numWrite;
    len -= numWrite;
    OMX_ERRORTYPE err = OMX_EmptyThisBuffer(ilclient_get_handle(client->decoder), buffer);
    assert(err == OMX_ErrorNone);
    // FIXME: We should check to see if there's a PortConfigChanged event 
    // instead of just looking at the first frame
    if (client->firstFrame) {
      setupDecoderRendererTunnel(client);
      client->firstFrame = 0;
    }
  }
  return 0;
}
