import { createSimpleEmitter } from "./util/SimpleEvent";

export class GameLoop {
  started = false;
  dead = false;
  running = false;
  milliPerFrame: number

  destroyEvent = createSimpleEmitter<[]>()

  constructor(
    fps: number,
    private loopFn: ()=>void
  ){
    this.milliPerFrame = 1000 / fps;
  }
    

  play(){
    if(this.dead){
      throw new Error("this loop is dead")
    }
    if(this.running){
      throw new Error("already running")
    }
    this.running = true
  }

  pause(){
    if(this.dead){
      throw new Error("this loop is dead")
    }
    if(!this.running){
      throw new Error("already paused")
    }
    this.running = false
  }

  destroy(){
    if(!this.dead) this.dead = true;
  }

  async start(){
    if(this.started){
      throw new Error("already started")
    }
    this.started = true;
    while(!this.dead){
      var start = Date.now();
      if(this.running) this.loopFn();
      await new Promise((res)=>{
        const diff = this.milliPerFrame - (Date.now() - start);
        if(diff <= 0) return res(void 0);
        setTimeout(res, diff)
      })
    }
    this.destroyEvent.emit();
  }

  async waitForEnd(){
    return new Promise<void>((res)=>{
      if(this.dead){
        return res();
      }
      this.destroyEvent(res);
    })
  }
}

function delay(time: number){
  const start = Date.now();
  return new Promise((res)=>{
    const diff = time - (Date.now() - start);
    if(diff <= 0) return res(void 0);
    setTimeout(res, diff)
  })
}
