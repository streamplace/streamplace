#include <string.h>
#include <mediamtx.h>
// #include "aquareum.h"

int main(int argc, char *argv[])
{
  if (argc < 2)
  {
    AquareumMain();
  }
  else if (strncmp("mediamtx", argv[1], 8) == 0)
  {
    MTXMain(argc, argv);
  }
  else
  {
    AquareumMain();
  }
}
