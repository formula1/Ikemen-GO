interface AudioSource {
  id: string;
  buffer: AudioBuffer;
  node: AudioBufferSourceNode;
  gainNode: GainNode;
  panNode: StereoPannerNode;
}

interface BGMState {
  source: AudioSource;
  loop: number;
  loopStart: number;
  loopEnd: number;
  volume: number;
  freqMul: number;
  paused: boolean;
}

interface SoundChannelState {
  source: AudioSource;
  group: number;
  number: number;
  volume: number;
  pan: number;
}

import { FileSystemAPI } from "../FileSystem";
import { BackgroundMusicConfig, BackgroundMusic } from "./BGM";
import { IAudioManager, IBackgroundMusic } from "./types";

export class AudioManager implements IAudioManager {
  public audioContext = new AudioContext();
  public masterGain = this.audioContext.createGain();
  public bgm: IBackgroundMusic | null = null;
  public channels: Map<string, SoundChannelState> = new Map();

  constructor(public fs: FileSystemAPI) {
    this.masterGain.connect(this.audioContext.destination);
  }

  async playBGM(config: BackgroundMusicConfig) {
    if (this.bgm) this.bgm.stop();
    this.bgm = await BackgroundMusic.create(this, config);
  }

  setBGMPaused(paused: boolean) {
    if(paused) this.bgm?.pause();
    else this.bgm?.resume();
  }

  setBGMVolume(volume: number) {
    if(this.bgm) this.bgm.setVolume(volume);
  }

  playSoundChannel(params: {
    channelId: string;
    soundId: string;
    group: number;
    number: number;
    loop: number;
    freqMul: number;
    loopStart: number;
    loopEnd: number;
    startPosition: number;
  }) {
      // Implementation for sound channel playback
      // Similar to BGM but with channel management
  }

  setChannelVolume(channelId: string, volume: number) {
    const channel = this.channels.get(channelId);
    if (channel) {
      channel.volume = volume;
      channel.source.gainNode.gain.value = volume;
    }
  }

  setMasterVolume(volume: number) {
    this.masterGain.gain.value = volume;
  }

  destroy(){
    this.audioContext.close();
  }
}