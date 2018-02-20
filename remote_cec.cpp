#include <cec.h>
#include <iostream>
#include <cassert>

#include "_cgo_export.h"
#include "remote_cec.h"


extern void go_callback_int(int foo, int p1);

using namespace CEC;

struct CECRemote {
	libcec_configuration config;
	ICECCallbacks callbacks;
	ICECAdapter* parser;
	int handleKeyPressCB;
	int handleCommandCB;
};


void CecLogMessage(void *cbParam, const cec_log_message* message)
{
	/*
    std::string strLevel;
    switch (message->level)
    {
    case CEC_LOG_ERROR:
      strLevel = "ERROR:   ";
      break;
    case CEC_LOG_WARNING:
      strLevel = "WARNING: ";
      break;
    case CEC_LOG_NOTICE:
      strLevel = "NOTICE:  ";
      break;
    case CEC_LOG_TRAFFIC:
      strLevel = "TRAFFIC: ";
      break;
    case CEC_LOG_DEBUG:
      strLevel = "DEBUG:   ";
      break;
    default:
      break;
    }

    std::string strFullLog;
    strFullLog = StringUtils::Format("%s[%16lld]\t%s", strLevel.c_str(), message->time, message->message);
		std::cout << strFullLog;
    PrintToStdOut(strFullLog.c_str());

    if (g_logOutput.is_open())
    {
      if (g_bShortLog)
        g_logOutput << message->message << std::endl;
      else
        g_logOutput << strFullLog.c_str() << std::endl;
    }
  }
	*/
}

void CecKeyPress(void* cbParam, const cec_keypress* key) {
	if (key->duration == 0) {
		return;
	}
	go_callback_int(((CECRemote*) cbParam)->handleKeyPressCB, key->keycode);
}

void CecCommand(void* cbParam, const cec_command* command)
{
	go_callback_int(((CECRemote*) cbParam)->handleCommandCB, command->opcode);
}

void CecAlert(void* cbParam, const libcec_alert type, const libcec_parameter param)
{
	/*
  switch (type)
  {
  case CEC_ALERT_CONNECTION_LOST:
    if (!CReconnect::Get().IsRunning())
    {
      PrintToStdOut("Connection lost - trying to reconnect\n");
      CReconnect::Get().CreateThread(false);
    }
    break;
  default:
    break;
  }
	*/
}

CECRemote* newCECRemote(int handleKeyPressCB, int handleCommandCB) {
	auto remote = new CECRemote();
	remote->handleKeyPressCB = handleKeyPressCB;
	remote->handleCommandCB = handleCommandCB;
  remote->config.Clear();
  snprintf(remote->config.strDeviceName, 13, "Jukybox");
  remote->config.clientVersion = LIBCEC_VERSION_CURRENT;
  remote->config.bActivateSource = 0;
  remote->config.deviceTypes.Add(CEC_DEVICE_TYPE_PLAYBACK_DEVICE);

  remote->callbacks.Clear();
  remote->callbacks.logMessage = &CecLogMessage;
  remote->callbacks.keyPress = &CecKeyPress;
  remote->callbacks.commandReceived = &CecCommand;
  remote->callbacks.alert = &CecAlert;
  remote->config.callbacks = &remote->callbacks;
	remote->config.callbackParam = remote;

  remote->parser = CECInitialise(&remote->config);
	assert(remote->parser);
	remote->parser->InitVideoStandalone();

	cec_adapter_descriptor devices[10];
  auto devicesFound = remote->parser->DetectAdapters(devices, 10, NULL, true);
	assert(devicesFound > 0);

  auto port = devices[0].strComName;
	std::cerr << "CEC: Found " << devicesFound << " devices. Opening " << port << std::endl;
  if (!remote->parser->Open(port)) {
		// TODO: Unload
		return nullptr;
	}

	return remote;
}

void deleteCECRemote(CECRemote* remote) {
	remote->parser->Close();
	CECDestroy(remote->parser);
	delete remote;
}
