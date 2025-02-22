
const IKEMEN_PREFIX = "IKEMEN-JS"
let instanceCount = 0;

import { FileSystemAPI } from "./FileSystem";
import { Graphics } from "./Graphics";
import { AudioManager } from "./Audio";
import { setupGOInstance } from "./wasm";
import { PlayerInput } from "./PlayerInput";
import { GameLoop } from "./GameLoop";

import { Util } from "./Util";

export class IkemenInstance {
  id = `${IKEMEN_PREFIX}-${(instanceCount++).toString(32)}`;

  public filesystem: FileSystemAPI;
  public graphics = new Graphics();
  public audio: AudioManager
  public playerInput = new PlayerInput(this.graphics.canvas);
  public loop = new GameLoop(60, ()=>{});

  public util = Util;

  constructor({ mugenBaseUrl }: { mugenBaseUrl: string }){
    this.filesystem = new FileSystemAPI(this.id, mugenBaseUrl);
    this.audio = new AudioManager(this.filesystem)
  }

  async start(){
    await this.filesystem.waitTillReady();
    await setupGOInstance(this.id);
    this.loop.start();
  }

  async waitForEnd(){
    return this.loop.waitForEnd();
  }

  destroy(){
    this.filesystem.destroy();
    this.graphics.destroy();
    this.audio.destroy();
    this.playerInput.destroy();
    this.loop.destroy();
  }
}

