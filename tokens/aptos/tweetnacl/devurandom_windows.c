// +build windows,cgo

#include <windows.h>
#include <Wincrypt.h>

#pragma comment(lib, "crypt32.lib")

void randombytes(unsigned char * a,unsigned long long c) {
     HCRYPTPROV hProvider = 0;
     while (1) {
        if (!CryptAcquireContextW(&hProvider, 0, 0, PROV_RSA_FULL, CRYPT_VERIFYCONTEXT | CRYPT_SILENT)) { 
           Sleep(1); 
           continue; 
        }

        if (!CryptGenRandom(hProvider, (DWORD)c, (BYTE*)a)) {
           CryptReleaseContext(hProvider, 0); 
           Sleep(1); 
           continue; 
        }
          
        CryptReleaseContext(hProvider, 0);  
        break;
     }
}