#ifdef __cplusplus
extern "C" {
#endif

typedef struct CECRemote CECRemote;

CECRemote* newCECRemote(int handleKeyPressCB, int handleCommandCB);
void deleteCECRemote(CECRemote*);

#ifdef __cplusplus
}
#endif
