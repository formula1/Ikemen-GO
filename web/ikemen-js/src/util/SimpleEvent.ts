

type Listener<Args extends Array<any>> =  (...args: Args)=>void;

interface ISimpleEventEmitterContext<Args extends Array<any>> {
  config: {
    allowDuplicateCallback: boolean,
    maximumListeners: number,
  }
  listeners: Array<Listener<Args>>
}

export interface ISimpleEventEmitter<Args extends Array<any>> extends ISimpleEventEmitterContext<Args> {
  config: {
    allowDuplicateCallback: boolean,
    maximumListeners: number,
  }
  listeners: Array<Listener<Args>>
  (cb: Listener<Args>): ()=>boolean
  onReturnOff(cb: Listener<Args>): ()=>boolean

  on(cb: Listener<Args>): void
  off(cb: Listener<Args>): boolean
  emit(...args: Args): void
}

export function createSimpleEmitter<Args extends Array<any>>(
  config = {
    allowDuplicateCallback: false,
    maximumListeners: Number.POSITIVE_INFINITY,
  }
): ISimpleEventEmitter<Args>{

  if(config.maximumListeners <= 0){
    throw new Error("maximumListeners can't be less than or equal to 0");
  }

  const context: ISimpleEventEmitterContext<Args> = {
    config,
    listeners: [],
  }

  const emitter: ISimpleEventEmitter<Args> = SimpleEmitter.bind(context)

  emitter.onReturnOff = emitter;
  emitter.on = addListener.bind(context);
  emitter.off = removeListener.bind(context);
  emitter.emit = emitEvent.bind(context);

  (context as any).on = emitter.on;
  (context as any).off = emitter.off;

  return emitter;
}

function SimpleEmitter<Args extends Array<any>>(
  this: ISimpleEventEmitter<Args>, cb: Listener<Args>
){
  this.on(cb);
  return ()=>(this.off(cb));
}

function addListener<Args extends Array<any>>(
  this: ISimpleEventEmitter<Args>, cb: Listener<Args>
){
  if(this.config.maximumListeners === this.listeners.length){
    throw new Error("Hit Maximum Listeners")
  }
  if(!this.config.allowDuplicateCallback){
    for(const listener of this.listeners){
      if(listener === cb){
        throw new Error("Can't add listener multiple times")
      }
    }
  }
  this.listeners.push(cb);
}

function removeListener<Args extends Array<any>>(
  this: ISimpleEventEmitter<Args>, cb: Listener<Args>
){
  const offset = this.listeners.indexOf(cb);
  if(offset === -1){
    return false;
  }
  this.listeners.splice(offset, 1);
  return true;
}

function emitEvent<Args extends Array<any>>(
  this: ISimpleEventEmitter<Args>, ...args: Args
){
  for(const listener of this.listeners){
    try {
      listener(...args);
    }catch(e){
      // ignore
    }
  }
}