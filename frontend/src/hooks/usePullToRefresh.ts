import { useEffect, useRef, type RefObject } from 'react'
import { isAndroid } from '../api'

// Pull to refresh (solo Android): misure in px dell'indicatore.
// PULL_THRESHOLD: discesa oltre cui il rilascio aggiorna; PULL_MAX: discesa
// massima; PULL_REST: posizione dell'indicatore mentre l'aggiornamento gira.
const PULL_THRESHOLD = 72
const PULL_MAX = 120
const PULL_REST = 56
const PULL_MIN_MS = 450

// usePullToRefresh: tirando giù il contenuto di scrollRef quando è già in
// cima, l'indicatore (indicatorRef, sotto l'header) scende e ruota; rilasciato
// oltre la soglia gira finché onRefresh non finisce. Con enabled falso il
// gesto non fa nulla, nemmeno l'animazione. Durante il trascinamento
// l'indicatore si muove direttamente sul DOM, senza ridisegnare l'App a ogni
// movimento; enabled e onRefresh stanno in ref per non dover registrare di
// nuovo i listener. mounted dice quando il contenitore che scorre esiste
// (prima del primo stato l'App non lo disegna).
export function usePullToRefresh(
    scrollRef: RefObject<HTMLDivElement | null>,
    indicatorRef: RefObject<HTMLDivElement | null>,
    mounted: boolean,
    enabled: boolean,
    onRefresh: () => Promise<void>,
) {
    const enabledRef = useRef(enabled)
    enabledRef.current = enabled
    const refreshRef = useRef(onRefresh)
    refreshRef.current = onRefresh

    useEffect(() => {
        const el = scrollRef.current
        if (!isAndroid || !el) return
        let startX = 0
        let startY = 0
        let tracking = false
        let pulling = false
        let refreshing = false
        let pull = 0

        function show(distance: number, animate: boolean) {
            const ind = indicatorRef.current
            if (!ind) return
            ind.style.transition = animate ? '' : 'none'
            ind.style.setProperty('--pull', distance + 'px')
            ind.style.setProperty('--pull-progress', String(Math.min(1, distance / PULL_THRESHOLD)))
            ind.classList.toggle('is-ready', distance >= PULL_THRESHOLD)
        }

        function onStart(e: TouchEvent) {
            tracking = false
            if (refreshing || !enabledRef.current || e.touches.length !== 1 || el!.scrollTop > 0) return
            tracking = true
            pulling = false
            startX = e.touches[0].clientX
            startY = e.touches[0].clientY
        }

        function onMove(e: TouchEvent) {
            if (!tracking) return
            const dx = e.touches[0].clientX - startX
            const dy = e.touches[0].clientY - startY
            if (!pulling) {
                // Si decide alla prima mossa netta: in giù con il contenuto in
                // cima è il gesto; altrimenti è uno scorrimento normale.
                if (Math.abs(dx) < 8 && Math.abs(dy) < 8) return
                if (dy <= 0 || Math.abs(dx) > dy || el!.scrollTop > 0) {
                    tracking = false
                    return
                }
                pulling = true
            }
            e.preventDefault()
            // Resistenza crescente: la discesa rallenta avvicinandosi al massimo.
            const raw = Math.max(0, dy)
            pull = PULL_MAX * (1 - Math.exp(-raw / (PULL_MAX * 1.6)))
            show(pull, false)
        }

        function onEnd() {
            if (!tracking) return
            tracking = false
            if (!pulling) return
            pulling = false
            if (pull < PULL_THRESHOLD || !enabledRef.current) {
                pull = 0
                show(0, true)
                return
            }
            refreshing = true
            show(PULL_REST, true)
            indicatorRef.current?.classList.add('is-refreshing')
            const start = performance.now()
            refreshRef
                .current()
                .catch(() => {})
                .then(() => new Promise((r) => window.setTimeout(r, Math.max(0, PULL_MIN_MS - (performance.now() - start)))))
                .then(() => {
                    refreshing = false
                    pull = 0
                    indicatorRef.current?.classList.remove('is-refreshing')
                    show(0, true)
                })
        }

        el.addEventListener('touchstart', onStart, { passive: true })
        el.addEventListener('touchmove', onMove, { passive: false })
        el.addEventListener('touchend', onEnd)
        el.addEventListener('touchcancel', onEnd)
        return () => {
            el.removeEventListener('touchstart', onStart)
            el.removeEventListener('touchmove', onMove)
            el.removeEventListener('touchend', onEnd)
            el.removeEventListener('touchcancel', onEnd)
        }
    }, [scrollRef, indicatorRef, mounted])
}
