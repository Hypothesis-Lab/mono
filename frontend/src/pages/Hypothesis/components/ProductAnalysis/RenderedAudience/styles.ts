import styled from 'styled-components'

export const Root = styled.div`
  display: flex;
  flex-direction: column;
  gap: 14px;
  padding: 14px;
  background: #fff;
`

export const AudienceHero = styled.div`
  display: flex;
  flex-direction: column;
  gap: 8px;
`

export const HeroStatus = styled.div`
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 9px;
`

export const StatusBadge = styled.span<{ $status: 'success' | 'warning' }>`
  display: inline-flex;
  align-items: center;
  min-height: 24px;
  padding: 2px 9px;
  border-radius: 999px;
  background: ${({ $status }) => ($status === 'success' ? '#e9f7ed' : '#fff4d6')};
  color: ${({ $status }) => ($status === 'success' ? '#21733a' : '#8a5a00')};
  font-size: 12px;
  font-weight: 700;
`

export const HeroCaption = styled.span`
  color: #777;
  font-size: 12px;
`

export const HeroText = styled.p`
  max-width: 760px;
  margin: 0;
  color: #444;
  font-size: 14px;
  line-height: 1.45;
`

export const SegmentList = styled.div`
  display: flex;
  flex-direction: column;
  gap: 10px;
`

export const SegmentCard = styled.details`
  border: 1px solid #e1e1e1;
  border-radius: 8px;
  background: #fcfcfc;

  &[open] > summary::before {
    transform: rotate(90deg);
  }
`

export const SegmentSummary = styled.summary`
  position: relative;
  padding: 13px 13px 13px 34px;
  border-radius: 8px;
  cursor: pointer;
  list-style: none;

  &::-webkit-details-marker {
    display: none;
  }

  &::before {
    position: absolute;
    top: 18px;
    left: 14px;
    content: '›';
    color: #666;
    font-size: 22px;
    line-height: 1;
    transition: transform 120ms ease;
  }

  &:hover {
    background: #f5f5f5;
  }
`

export const SegmentBody = styled.div`
  display: flex;
  flex-direction: column;
  gap: 11px;
  padding: 0 13px 13px;
`

export const SegmentHeader = styled.div`
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 10px;
`

export const SegmentName = styled.h4`
  margin: 0;
  color: #292929;
  font-size: 16px;
`

export const SegmentId = styled.span`
  display: block;
  margin-top: 3px;
  color: #8a8a8a;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 11px;
`

export const SegmentBadges = styled.div`
  display: flex;
  flex-wrap: wrap;
  justify-content: flex-end;
  gap: 5px;
`

export const SelectionBadge = styled.span`
  padding: 4px 7px;
  border-radius: 4px;
  background: #e9f7ed;
  color: #21733a;
  font-size: 11px;
  font-weight: 700;
`

export const SizeBadge = styled.span`
  padding: 4px 7px;
  border-radius: 4px;
  background: #f0f0f0;
  color: #555;
  font-size: 11px;
`

export const SegmentText = styled.p`
  margin: 0;
  color: #444;
  font-size: 14px;
  line-height: 1.45;
`

export const SegmentBlock = styled.section`
  display: flex;
  flex-direction: column;
  gap: 7px;
`

export const BlockTitle = styled.h5`
  margin: 0;
  color: #555;
  font-size: 12px;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.02em;
`

export const BulletList = styled.ul`
  display: flex;
  flex-direction: column;
  gap: 5px;
  margin: 0;
  padding-left: 17px;
  color: #555;
  font-size: 12px;
  line-height: 1.4;
`

export const TableWrapper = styled.div`
  max-width: 100%;
  overflow-x: auto;
`

export const FeatureTable = styled.table`
  width: 100%;
  min-width: 620px;
  border-collapse: collapse;
  color: #444;
  font-size: 12px;

  th, td {
    padding: 8px;
    border-bottom: 1px solid #e9e9e9;
    text-align: left;
    vertical-align: top;
  }

  th {
    color: #888;
    font-size: 11px;
    font-weight: 600;
  }
`

export const FeatureName = styled.span`
  display: block;
  color: #444;
  font-weight: 600;
`

export const FeatureValue = styled.span`
  display: block;
  margin-top: 3px;
  color: #888;
  font-size: 11px;
`

export const Direction = styled.span<{ $higher: boolean }>`
  display: block;
  color: ${({ $higher }) => ($higher ? '#21733a' : '#8a5a00')};
  font-weight: 700;
`

export const Effect = styled.span`
  display: block;
  margin-top: 3px;
  color: #999;
  font-size: 11px;
`

export const Unknowns = styled.section`
  padding-top: 10px;
  border-top: 1px solid #e9e9e9;
`

export const ReviewPanel = styled.section`
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding: 12px;
  border: 1px solid #dce4ef;
  border-radius: 7px;
  background: #f7faff;
`

export const LimitationPanel = styled.section`
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding: 12px;
  border: 1px solid #ead8a4;
  border-radius: 7px;
  background: #fffaf0;
`

export const EmptyState = styled.p`
  margin: 0;
  color: #777;
  font-size: 13px;
`
