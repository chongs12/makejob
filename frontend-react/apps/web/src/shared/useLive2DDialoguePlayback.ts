import { useEffect, useRef, useState } from 'react'

export interface Live2DDialoguePlaybackOptions {
  initialDialogue: string
  onPlaybackFinished?: () => void
  onPlaybackError?: (error: unknown) => void
}

export interface Live2DDialoguePlaybackController {
  liveDialogue: string
  isDialogueTyping: boolean
  mouthOpen: number
  stopDialogueTyping: (finalText?: string) => void
  startDialogueTyping: (text: string, audio?: HTMLAudioElement | null) => void
  stopCurrentPlayback: (finalDialogue?: string) => void
  syncDialogueImmediately: (text: string) => void
  playTTSAudio: (audioUrl: string, text: string) => Promise<void>
}

/**
 * 将舞台字幕文本拆成逐步显示的最小单元，兼容中文字符和常见 emoji。
 */
export function splitLive2DDialogueUnits(text: string): string[] {
  return Array.from(text)
}

/**
 * 按文本长度与标点停顿粗略估算字幕播放时长，供无音频时兜底同步。
 */
export function estimateLive2DDialogueDurationMs(text: string): number {
  const units = splitLive2DDialogueUnits(text)
  const punctuationCount = units.filter((unit) => /[，。！？；：,.!?]/.test(unit)).length
  const baseDurationMs = units.length * 110 + punctuationCount * 140
  return Math.min(Math.max(baseDurationMs, 1400), 12000)
}

/**
 * 统一管理 Live2D 舞台字幕、TTS 播放与嘴型同步，供面试页和陪伴页复用。
 */
export function useLive2DDialoguePlayback(options: Live2DDialoguePlaybackOptions): Live2DDialoguePlaybackController {
  const [liveDialogue, setLiveDialogue] = useState(options.initialDialogue)
  const [isDialogueTyping, setIsDialogueTyping] = useState(false)
  const [mouthOpen, setMouthOpen] = useState(0)
  const audioContextRef = useRef<AudioContext | null>(null)
  const analyserRef = useRef<AnalyserNode | null>(null)
  const analyserFrameRef = useRef<number | null>(null)
  const audioElementRef = useRef<HTMLAudioElement | null>(null)
  const dialogueFrameRef = useRef<number | null>(null)
  const dialoguePlaybackTokenRef = useRef(0)
  const audioEndedGuardRef = useRef(false)
  const suppressAudioErrorRef = useRef(false)

  /**
   * 停止当前字幕动画，并按需把舞台文案直接收敛到目标文本。
   */
  function stopDialogueTyping(finalText?: string): void {
    dialoguePlaybackTokenRef.current += 1
    if (dialogueFrameRef.current) {
      window.cancelAnimationFrame(dialogueFrameRef.current)
      dialogueFrameRef.current = null
    }
    setIsDialogueTyping(false)
    if (typeof finalText === 'string') {
      setLiveDialogue(finalText)
    }
  }

  /**
   * 按音频进度或兜底估时推进字幕显示，营造近似跟读的打字机效果。
   */
  function startDialogueTyping(text: string, audio?: HTMLAudioElement | null): void {
    const normalizedText = text.trim()
    stopDialogueTyping()

    if (!normalizedText) {
      setLiveDialogue('')
      return
    }

    const units = splitLive2DDialogueUnits(normalizedText)
    const fallbackDurationMs = estimateLive2DDialogueDurationMs(normalizedText)
    const playbackToken = dialoguePlaybackTokenRef.current + 1
    const startedAt = window.performance.now()
    let lastVisibleCount = -1

    dialoguePlaybackTokenRef.current = playbackToken
    audioEndedGuardRef.current = false
    setIsDialogueTyping(true)
    setLiveDialogue('')

    /**
     * 逐帧根据音频进度或兜底时长计算当前应显示的字幕长度。
     */
    function syncDialogueFrame(): void {
      if (dialoguePlaybackTokenRef.current !== playbackToken) {
        return
      }

      const audioDurationMs = audio && Number.isFinite(audio.duration) && audio.duration > 0 ? audio.duration * 1000 : 0
      const totalDurationMs = audioDurationMs || fallbackDurationMs
      const elapsedMs = audio
        ? Math.max(audio.currentTime * 1000, window.performance.now() - startedAt)
        : window.performance.now() - startedAt
      const progress = totalDurationMs > 0 ? Math.min(elapsedMs / totalDurationMs, 1) : 1
      const visibleCount = progress >= 1 ? units.length : Math.max(1, Math.ceil(units.length * progress))

      if (visibleCount !== lastVisibleCount) {
        lastVisibleCount = visibleCount
        setLiveDialogue(units.slice(0, visibleCount).join(''))
      }

      if (visibleCount >= units.length) {
        dialogueFrameRef.current = null
        setIsDialogueTyping(false)
        setLiveDialogue(normalizedText)
        return
      }

      dialogueFrameRef.current = window.requestAnimationFrame(syncDialogueFrame)
    }

    dialogueFrameRef.current = window.requestAnimationFrame(syncDialogueFrame)
  }

  /**
   * 停止上一段音频并释放分析器资源，避免多个语音上下文叠加。
   */
  function stopCurrentPlayback(finalDialogue?: string): void {
    if (analyserFrameRef.current) {
      window.cancelAnimationFrame(analyserFrameRef.current)
      analyserFrameRef.current = null
    }
    analyserRef.current = null
    if (audioElementRef.current) {
      suppressAudioErrorRef.current = true
      audioElementRef.current.onended = null
      audioElementRef.current.onerror = null
      audioElementRef.current.pause()
      audioElementRef.current.src = ''
      audioElementRef.current = null
    }
    if (audioContextRef.current) {
      void audioContextRef.current.close()
      audioContextRef.current = null
    }
    stopDialogueTyping(finalDialogue)
    setMouthOpen(0)
  }

  /**
   * 将当前文案立即同步到舞台，适合非流式场景快速刷新字幕。
   */
  function syncDialogueImmediately(text: string): void {
    stopCurrentPlayback(text.trim())
  }

  /**
   * 播放新的 TTS 音频，并用音频分析器驱动 Live2D 嘴型与字幕同步。
   */
  async function playTTSAudio(audioUrl: string, text: string): Promise<void> {
    const dialogueText = text.trim()

    stopCurrentPlayback()

    if (!dialogueText) {
      return
    }

    if (!audioUrl) {
      startDialogueTyping(dialogueText)
      options.onPlaybackFinished?.()
      return
    }

    try {
      const AudioContextCtor = window.AudioContext
      if (!AudioContextCtor) {
        startDialogueTyping(dialogueText)
        options.onPlaybackError?.(new Error('当前浏览器不支持音频上下文，已回退到文本模式。'))
        options.onPlaybackFinished?.()
        return
      }

      const audio = new Audio(audioUrl)
      audio.preload = 'auto'
      audioElementRef.current = audio

      const audioContext = new AudioContextCtor()
      const analyser = audioContext.createAnalyser()
      analyser.fftSize = 2048
      const source = audioContext.createMediaElementSource(audio)
      source.connect(analyser)
      analyser.connect(audioContext.destination)
      audioContextRef.current = audioContext
      analyserRef.current = analyser

      /**
       * 读取当前音频振幅，并持续同步到嘴型开合值。
       */
      function syncMouthFromAudio(): void {
        if (!analyserRef.current) {
          setMouthOpen(0)
          return
        }

        const analyserNode = analyserRef.current
        const buffer = new Uint8Array(analyserNode.fftSize)
        analyserNode.getByteTimeDomainData(buffer)
        let sum = 0
        for (const value of buffer) {
          sum += Math.abs(value - 128)
        }
        const normalized = Math.min(sum / buffer.length / 26, 1)
        setMouthOpen(normalized)
        analyserFrameRef.current = window.requestAnimationFrame(syncMouthFromAudio)
      }

      audio.onended = () => {
        if (audioEndedGuardRef.current) {
          return
        }
        audioEndedGuardRef.current = true
        stopCurrentPlayback(dialogueText)
        options.onPlaybackFinished?.()
      }
      audio.onerror = () => {
        if (suppressAudioErrorRef.current) {
          suppressAudioErrorRef.current = false
          return
        }
        stopCurrentPlayback()
        startDialogueTyping(dialogueText)
        options.onPlaybackError?.(new Error('语音资源播放失败，已回退到文本模式。'))
        options.onPlaybackFinished?.()
      }

      await audioContext.resume()
      suppressAudioErrorRef.current = false
      analyserFrameRef.current = window.requestAnimationFrame(syncMouthFromAudio)
      await audio.play()
      startDialogueTyping(dialogueText, audio)
    } catch (error) {
      suppressAudioErrorRef.current = false
      stopCurrentPlayback()
      startDialogueTyping(dialogueText)
      options.onPlaybackError?.(error)
      options.onPlaybackFinished?.()
    }
  }

  /**
   * 在页面卸载时清理音频与字幕动画资源，避免残留占用浏览器上下文。
   */
  useEffect(() => {
    return () => {
      stopCurrentPlayback()
    }
  }, [])

  return {
    liveDialogue,
    isDialogueTyping,
    mouthOpen,
    stopDialogueTyping,
    startDialogueTyping,
    stopCurrentPlayback,
    syncDialogueImmediately,
    playTTSAudio,
  }
}
