import { FileSystemAPI } from "../FileSystem";

import { IAudioManager, AudioSource, IBackgroundMusic } from "./types";

export type BackgroundMusicConfig = {
  filename: string;
  loop: number;
  volume: number;
  loopStart: number;
  loopEnd: number;
  startPosition: number;
  freqMul: number;
};

export class BackgroundMusic implements IBackgroundMusic {
  public paused: boolean = false;
  constructor(
    private manager: IAudioManager,
    private config: BackgroundMusicConfig,
    private sourceNode: AudioSource
  ){}

  static async create(
    manager: IAudioManager,
    config: BackgroundMusicConfig
  ){
    const arrayBuffer = manager.fs.readFile(config.filename);
    if(!arrayBuffer) throw new Error("Missing File")
    const audioBuffer = await manager.audioContext.decodeAudioData(arrayBuffer);
  
    const audioSource = setupAudioSource(manager, audioBuffer, config);
  
    audioSource.gainNode.gain.value = config.volume / 100;
    audioSource.node.start(0, config.startPosition);
  
    return new BackgroundMusic(manager, config, audioSource);
  }

  stop(){
    this.sourceNode.node.stop();
  }

  pause(){
    this.sourceNode.node.stop();
    this.paused = true;
  }

  resume(){
    this.sourceNode.node = createAudioSource(
      this.manager, this.sourceNode.buffer, this.config
    );
    
    this.sourceNode.node.connect(this.sourceNode.gainNode);
    this.sourceNode.node.start();

    this.paused = false;
  }

  setVolume(volume: number){
    this.sourceNode.gainNode.gain.value = volume / 100;
  }
}

function setupAudioSource(
  manager: IAudioManager, audioBuffer: AudioBuffer, config: BackgroundMusicConfig,
): AudioSource {
  const source = createAudioSource(manager, audioBuffer, config);
  const gainNode = manager.audioContext.createGain();
  const panNode = manager.audioContext.createStereoPanner();

  source.connect(gainNode);
  gainNode.connect(panNode);
  panNode.connect(this.masterGain);

  return {
    id: crypto.randomUUID(),
    buffer: audioBuffer,
    node: source,
    gainNode,
    panNode
  };
}

function createAudioSource(
  manager: IAudioManager,
  audioBuffer: AudioBuffer,
  config: BackgroundMusicConfig,
){
  const source = manager.audioContext.createBufferSource();
  source.buffer = audioBuffer;
  source.loop = config.loop !== 0;
  source.loopStart = config.loopStart;
  source.loopEnd = config.loopEnd;
  source.playbackRate.value = config.freqMul;
  return source
}
