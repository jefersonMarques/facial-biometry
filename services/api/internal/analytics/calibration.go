package analytics

import (
    "context"
    "database/sql"
    "encoding/json"
    "errors"
    "math"
    "sort"
    "time"
)

const CalibrationBins = 20
const CalibrationMaxWindow = 90 * 24 * time.Hour

type CalibrationReport struct {
    From time.Time `json:"from"`
    To time.Time `json:"to"`
    RuntimeFilter string `json:"runtimeFilter,omitempty"`
    CompletedCNH int64 `json:"completedCnh"`
    WithBenchmark int64 `json:"withBenchmark"`
    Approved int64 `json:"approved"`
    Rejected int64 `json:"rejected"`
    Review int64 `json:"review"`
    UnknownTruth int64 `json:"unknownTruth"`
    ConfirmedGenuine int64 `json:"confirmedGenuine"`
    ConfirmedImpostor int64 `json:"confirmedImpostor"`
    Series []CalibrationSeries `json:"series"`
    MethodHealth []CalibrationMethod `json:"methodHealth"`
    Runtimes []CalibrationRuntime `json:"runtimes"`
    Calibrated bool `json:"calibrated"`
}
type CalibrationSeries struct {
    Method string `json:"method"`
    Pair string `json:"pair"`
    TruthLabel string `json:"truthLabel"`
    Attempts int64 `json:"attempts"`
    Compared int64 `json:"compared"`
    MeanCosine *float64 `json:"meanCosine,omitempty"`
    Histogram []int64 `json:"histogram"`
}
type CalibrationMethod struct {
    Method string `json:"method"`
    Attempts int64 `json:"attempts"`
    Compared int64 `json:"compared"`
    Failed int64 `json:"failed"`
    AvgDurationMS float64 `json:"avgDurationMs"`
}
type CalibrationRuntime struct {
    RuntimeID string `json:"runtimeId"`
    EmbeddingModel string `json:"embeddingModel"`
    Completed int64 `json:"completed"`
}

func ValidateCalibrationTruth(label, source string) error {
    if label == "unknown" && source == "" { return nil }
    if (label == "genuine_confirmed" || label == "impostor_confirmed") &&
        (source == "controlled_test" || source == "external_verified" || source == "manual_review") { return nil }
    return errors.New("truth label requires independent confirmation and valid evidence source")
}

func (r *Repository) UpdateCalibrationTruth(ctx context.Context, id, label, source string) error {
    if err := ValidateCalibrationTruth(label,source); err != nil { return err }
    tx,err:=r.db.BeginTx(ctx,nil)
    if err!=nil {return err}
    defer tx.Rollback()
    var previousLabel,previousSource string
    err=tx.QueryRowContext(ctx,
        "SELECT truth_label,truth_source FROM faceproof_checks WHERE check_id=$1 AND document IS NOT NULL FOR UPDATE",
        id).Scan(&previousLabel,&previousSource)
    if errors.Is(err,sql.ErrNoRows) {return ErrNotFound}
    if err!=nil {return err}
    if previousLabel==label && previousSource==source {return tx.Commit()}
    _,err=tx.ExecContext(ctx,"UPDATE faceproof_checks SET truth_label=$2,truth_source=$3,truth_updated_at=NOW(),updated_at=NOW() WHERE check_id=$1",id,label,source)
    if err!=nil {return err}
    payload,err:=json.Marshal(map[string]string{
        "beforeLabel":previousLabel,"beforeSource":previousSource,
        "afterLabel":label,"afterSource":source,
    })
    if err!=nil {return err}
    _,err=tx.ExecContext(ctx,
        "INSERT INTO faceproof_events (check_id,event_type,payload) VALUES ($1,'admin_ground_truth_updated',$2::jsonb)",
        id,string(payload))
    if err!=nil {return err}
    return tx.Commit()
}

func (r *Repository) Calibration(ctx context.Context, from,to time.Time, runtime string) (CalibrationReport,error) {
    if from.IsZero() || to.IsZero() || !from.Before(to) ||
        to.Sub(from)>CalibrationMaxWindow || len(runtime)>128 {
        return CalibrationReport{}, errors.New("invalid calibration period or runtime")
    }
    from,to = from.UTC(),to.UTC()
    report:=CalibrationReport{
        From:from,To:to,RuntimeFilter:runtime,
        Series:make([]CalibrationSeries,0),MethodHealth:make([]CalibrationMethod,0),
        Runtimes:make([]CalibrationRuntime,0),
    }
    const counts=`WITH checks AS (
        SELECT c.check_id,c.actual_decision,c.truth_label
        FROM faceproof_checks c
        WHERE c.document IS NOT NULL AND c.completed_at >= $1::TIMESTAMPTZ
        AND c.completed_at < $2::TIMESTAMPTZ
        AND ($3::TEXT = '' OR COALESCE(c.runtime->>'runtimeId','unknown') = $3)
    )
    SELECT COUNT(*),
      COUNT(*) FILTER(WHERE actual_decision='approved'),
      COUNT(*) FILTER(WHERE actual_decision='rejected'),
      COUNT(*) FILTER(WHERE actual_decision='review'),
      COUNT(*) FILTER(WHERE truth_label='unknown'),
      COUNT(*) FILTER(WHERE truth_label='genuine_confirmed'),
      COUNT(*) FILTER(WHERE truth_label='impostor_confirmed'),
      COUNT(*) FILTER(WHERE EXISTS (SELECT 1 FROM faceproof_events e
          WHERE e.check_id=checks.check_id AND e.event_type='cnh_low_resolution_shadow'))
    FROM checks`
    if err:=r.db.QueryRowContext(ctx,counts,from,to,runtime).Scan(
        &report.CompletedCNH,&report.Approved,&report.Rejected,&report.Review,
        &report.UnknownTruth,&report.ConfirmedGenuine,&report.ConfirmedImpostor,
        &report.WithBenchmark);err!=nil{return CalibrationReport{},err}

    const versions=`SELECT COALESCE(runtime->>'runtimeId','unknown'), embedding_model,COUNT(*)
        FROM faceproof_checks WHERE document IS NOT NULL
        AND completed_at >= $1::TIMESTAMPTZ AND completed_at < $2::TIMESTAMPTZ
        GROUP BY 1,2 ORDER BY COUNT(*) DESC,1,2 LIMIT 50`
    rows,err:=r.db.QueryContext(ctx,versions,from,to)
    if err!=nil{return CalibrationReport{},err}
    for rows.Next(){
        var x CalibrationRuntime
        if err=rows.Scan(&x.RuntimeID,&x.EmbeddingModel,&x.Completed);err!=nil{break}
        report.Runtimes=append(report.Runtimes,x)
    }
    if err==nil{err=rows.Err()}
    rows.Close()
    if err!=nil{return CalibrationReport{},err}

    // Latest event only, no check ID/photo/embedding crosses the API boundary.
    const samples=`WITH latest AS (
        SELECT DISTINCT ON(e.check_id) e.check_id,e.payload,c.truth_label
        FROM faceproof_events e JOIN faceproof_checks c ON c.check_id=e.check_id
        WHERE e.event_type='cnh_low_resolution_shadow'
          AND c.document IS NOT NULL
          AND c.completed_at >= $1::TIMESTAMPTZ
          AND c.completed_at < $2::TIMESTAMPTZ
          AND ($3::TEXT = '' OR COALESCE(c.runtime->>'runtimeId','unknown')=$3)
        ORDER BY e.check_id,e.id DESC
    ), variants AS (
        SELECT l.truth_label,l.payload,v.item,v.item->>'method' AS method
        FROM latest l
        CROSS JOIN LATERAL jsonb_array_elements(
          CASE WHEN jsonb_typeof(l.payload->'variants')='array'
          THEN l.payload->'variants' ELSE '[]'::jsonb END
        ) AS v(item)
        WHERE v.item->>'method' IN ('production_cubic','nearest','linear','lanczos4')
    )`

    // 20 bins across [-1,1]; null/invalid results are counted as failed.
    const histogram=samples+`, points AS (
        SELECT v.method,v.truth_label,p.pair,
        CASE WHEN p.metric->>'status'='compared'
            AND jsonb_typeof(p.metric->'robustCosine')='number'
            THEN (p.metric->>'robustCosine')::DOUBLE PRECISION ELSE NULL END AS cosine
        FROM variants v CROSS JOIN LATERAL (VALUES
            ('pdf_vio',v.item->'pdfVio'),
            ('vio_live',v.item->'vioLive'),
            ('pdf_live',v.payload->'pdf_live')
        ) AS p(pair,metric)
        WHERE p.pair <> 'pdf_live' OR v.method = 'production_cubic'
    ), bucketed AS (
        SELECT method,pair,truth_label,
            CASE WHEN cosine IS NULL OR cosine < -1 OR cosine > 1 THEN NULL
                 ELSE cosine END AS valid_cosine
        FROM points
    )
    SELECT method,pair,truth_label,COUNT(*),COUNT(valid_cosine),
        COALESCE(SUM(valid_cosine),0),
        CASE WHEN valid_cosine IS NULL THEN -1
             ELSE LEAST(19,GREATEST(0,FLOOR((valid_cosine+1)*10)::INTEGER))
        END AS bucket
    FROM bucketed GROUP BY method,pair,truth_label,bucket
    ORDER BY method,pair,truth_label,bucket`
    rows,err=r.db.QueryContext(ctx,histogram,from,to,runtime)
    if err!=nil{return CalibrationReport{},err}
    groups:=map[string]*CalibrationSeries{}
    totals:=map[string]float64{}
    for rows.Next(){
        var method,pair,truth string
        var attempts,compared int64
        var sum float64
        var bucket int
        if err=rows.Scan(&method,&pair,&truth,&attempts,&compared,&sum,&bucket);err!=nil{break}
        key:=method+"|"+pair+"|"+truth
        x:=groups[key]
        if x==nil {
            x=&CalibrationSeries{Method:method,Pair:pair,TruthLabel:truth,
                Histogram:make([]int64,CalibrationBins)}
            groups[key]=x
        }
        x.Attempts+=attempts
        x.Compared+=compared
        if bucket>=0 && bucket<CalibrationBins {
            x.Histogram[bucket]+=compared
            totals[key]+=sum
        }
    }
    if err==nil{err=rows.Err()}
    rows.Close()
    if err!=nil{return CalibrationReport{},err}
    for key,x:=range groups {
        if x.Compared>0 {
            mean:=totals[key]/float64(x.Compared)
            if !math.IsNaN(mean) && !math.IsInf(mean,0) {x.MeanCosine=&mean}
        }
        report.Series=append(report.Series,*x)
    }
    sort.Slice(report.Series,func(i,j int)bool{
        a,b:=report.Series[i],report.Series[j]
        if a.Pair!=b.Pair{return a.Pair<b.Pair}
        if a.Method!=b.Method{return a.Method<b.Method}
        return a.TruthLabel<b.TruthLabel
    })

    const methods=samples+` SELECT method,COUNT(*),
       COUNT(*) FILTER (WHERE item->>'status'='compared'),
       COUNT(*) FILTER (WHERE item->>'status' IS DISTINCT FROM 'compared'),
       COALESCE(AVG(CASE WHEN jsonb_typeof(item->'durationMs')='number'
       THEN (item->>'durationMs')::DOUBLE PRECISION END),0)
       FROM variants GROUP BY method ORDER BY method`
    rows,err=r.db.QueryContext(ctx,methods,from,to,runtime)
    if err!=nil{return CalibrationReport{},err}
    for rows.Next(){
        var x CalibrationMethod
        if err=rows.Scan(&x.Method,&x.Attempts,&x.Compared,&x.Failed,&x.AvgDurationMS);err!=nil{break}
        report.MethodHealth=append(report.MethodHealth,x)
    }
    if err==nil{err=rows.Err()}
    rows.Close()
    return report,err
}

func (r *Repository) CalibrationTruth(ctx context.Context,id string) (string,string,error) {
    var label,source string
    err:=r.db.QueryRowContext(ctx,
        "SELECT truth_label,truth_source FROM faceproof_checks WHERE check_id=$1 AND document IS NOT NULL",
        id).Scan(&label,&source)
    if errors.Is(err,sql.ErrNoRows) {return "","",ErrNotFound}
    return label,source,err
}
